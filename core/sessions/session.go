package sessions

import (
	"SI-MQTT/core/message"
	"fmt"
	"sync"
)

const (
	// Queue size for the ack queue
	//队列的队列大小
	defaultQueueSize = 1024 >> 2
)

// 客户端会话
type Session struct {
	// Ack queue for outgoing PUBLISH QoS 1 messages
	//用于传出发布QoS 1消息的Ack队列
	Pub1ack *Ackqueue

	// Ack queue for incoming PUBLISH QoS 2 messages
	//传入发布QoS 2消息的Ack队列
	Pub2in *Ackqueue

	// Ack queue for outgoing PUBLISH QoS 2 messages
	//用于传出发布QoS 2消息的Ack队列
	Pub2out *Ackqueue

	// Ack queue for outgoing SUBSCRIBE messages
	//用于发送订阅消息的Ack队列
	Suback *Ackqueue

	// Ack queue for outgoing UNSUBSCRIBE messages
	//发送取消订阅消息的Ack队列
	Unsuback *Ackqueue

	// Ack queue for outgoing PINGREQ messages
	//用于发送PINGREQ消息的Ack队列
	Pingack *Ackqueue

	// cmsg is the CONNECT message
	//cmsg是连接消息
	Cmsg *message.ConnectMessage

	// Will message to publish if connect is closed unexpectedly
	//如果连接意外关闭，遗嘱消息将发布
	Will *message.PublishMessage

	// Retained publish message
	//保留发布消息
	Retained *message.PublishMessage

	// cbuf is the CONNECT message buffer, this is for storing all the will stuff
	//cbuf是连接消息缓冲区，用于存储所有的will内容
	cbuf []byte

	// rbuf is the retained PUBLISH message buffer
	// rbuf是保留的发布消息缓冲区
	rbuf []byte

	// topics stores all the topis for this session/client
	//主题存储此会话/客户机的所有topics
	topics map[string]byte

	// Initialized?
	initted bool

	// Serialize access to this session
	//序列化对该会话的访问锁
	mu sync.Mutex

	id string
}

func (this *Session) Init(msg *message.ConnectMessage) error {
	this.mu.Lock()
	defer this.mu.Unlock()

	if this.initted {
		return fmt.Errorf("Session already initialized")
	}

	this.cbuf = make([]byte, msg.Len())
	this.Cmsg = message.NewConnectMessage()

	if _, err := msg.Encode(this.cbuf); err != nil {
		return err
	}

	if _, err := this.Cmsg.Decode(this.cbuf); err != nil {
		return err
	}

	if this.Cmsg.WillFlag() {
		this.Will = message.NewPublishMessage()
		this.Will.SetQoS(this.Cmsg.WillQos())
		this.Will.SetTopic(this.Cmsg.WillTopic())
		this.Will.SetPayload(this.Cmsg.WillMessage())
		this.Will.SetRetain(this.Cmsg.WillRetain())
	}

	this.topics = make(map[string]byte, 1)

	this.id = string(msg.ClientId())

	this.Pub1ack = newAckqueue(defaultQueueSize)
	this.Pub2in = newAckqueue(defaultQueueSize)
	this.Pub2out = newAckqueue(defaultQueueSize)
	this.Suback = newAckqueue(defaultQueueSize << 2)
	this.Unsuback = newAckqueue(defaultQueueSize << 2)
	this.Pingack = newAckqueue(defaultQueueSize << 2)

	this.initted = true

	return nil
}

func (this *Session) Update(msg *message.ConnectMessage) error {
	this.mu.Lock()
	defer this.mu.Unlock()

	this.cbuf = make([]byte, msg.Len())
	this.Cmsg = message.NewConnectMessage()

	if _, err := msg.Encode(this.cbuf); err != nil {
		return err
	}

	if _, err := this.Cmsg.Decode(this.cbuf); err != nil {
		return err
	}

	return nil
}

func (this *Session) RetainMessage(msg *message.PublishMessage) error {
	this.mu.Lock()
	defer this.mu.Unlock()

	this.rbuf = make([]byte, msg.Len())
	this.Retained = message.NewPublishMessage()

	if _, err := msg.Encode(this.rbuf); err != nil {
		return err
	}

	if _, err := this.Retained.Decode(this.rbuf); err != nil {
		return err
	}

	return nil
}

func (this *Session) AddTopic(topic string, qos byte) error {
	this.mu.Lock()
	defer this.mu.Unlock()

	if !this.initted {
		return fmt.Errorf("Session not yet initialized")
	}

	this.topics[topic] = qos

	return nil
}

func (this *Session) RemoveTopic(topic string) error {
	this.mu.Lock()
	defer this.mu.Unlock()

	if !this.initted {
		return fmt.Errorf("Session not yet initialized")
	}

	delete(this.topics, topic)

	return nil
}

func (this *Session) Topics() ([]string, []byte, error) {
	this.mu.Lock()
	defer this.mu.Unlock()

	if !this.initted {
		return nil, nil, fmt.Errorf("Session not yet initialized")
	}

	var (
		topics []string
		qoss   []byte
	)

	for k, v := range this.topics {
		topics = append(topics, k)
		qoss = append(qoss, v)
	}

	return topics, qoss, nil
}

func (this *Session) ID() string {
	return string(this.Cmsg.ClientId())
}
