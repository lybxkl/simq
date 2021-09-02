package sessionsv5

import (
	"fmt"
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"sync"
)

const (
	// Queue size for the ack queue
	//队列的队列大小
	defaultQueueSize = 1024 >> 2
)

// 客户端会话
type session struct {
	// Ack queue for outgoing PUBLISH QoS 1 messages
	//用于传出发布QoS 1消息的Ack队列
	pub1ack Ackqueue

	// Ack queue for incoming PUBLISH QoS 2 messages
	//传入发布QoS 2消息的Ack队列
	pub2in Ackqueue

	// Ack queue for outgoing PUBLISH QoS 2 messages
	//用于传出发布QoS 2消息的Ack队列
	pub2out Ackqueue

	// Ack queue for outgoing SUBSCRIBE messages
	//用于发送订阅消息的Ack队列
	suback Ackqueue

	// Ack queue for outgoing UNSUBSCRIBE messages
	//发送取消订阅消息的Ack队列
	unsuback Ackqueue

	// Ack queue for outgoing PINGREQ messages
	//用于发送PINGREQ消息的Ack队列
	pingack Ackqueue

	// cmsg is the CONNECT messagev5
	//cmsg是连接消息
	cmsg *messagev5.ConnectMessage

	// Will messagev5 to publish if connect is closed unexpectedly
	//如果连接意外关闭，遗嘱消息将发布
	will *messagev5.PublishMessage

	// cbuf is the CONNECT messagev5 buffer, this is for storing all the will stuff
	//cbuf是连接消息缓冲区，用于存储所有的will内容
	cbuf []byte

	// rbuf is the retained PUBLISH messagev5 buffer
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

func NewMemSession() Session {
	return &session{}
}

//Init 遗嘱和connect消息会存在每个session中，不用每次查询数据库的
func (this *session) Init(msg *messagev5.ConnectMessage, topics ...SessionInitTopic) error {
	this.mu.Lock()
	defer this.mu.Unlock()

	if this.initted {
		return fmt.Errorf("session already initialized")
	}

	this.cbuf = make([]byte, msg.Len())
	this.cmsg = messagev5.NewConnectMessage()

	if _, err := msg.Encode(this.cbuf); err != nil {
		return err
	}

	if _, err := this.cmsg.Decode(this.cbuf); err != nil {
		return err
	}

	if this.cmsg.WillFlag() {
		this.will = messagev5.NewPublishMessage()
		this.will.SetQoS(this.cmsg.WillQos())
		this.will.SetTopic(this.cmsg.WillTopic())
		this.will.SetPayload(this.cmsg.WillMessage())
		this.will.SetRetain(this.cmsg.WillRetain())
	}

	this.topics = make(map[string]byte, 1)

	this.id = string(msg.ClientId())

	this.pub1ack = newAckqueue(defaultQueueSize)
	this.pub2in = newAckqueue(defaultQueueSize)
	this.pub2out = newAckqueue(defaultQueueSize)
	this.suback = newAckqueue(defaultQueueSize << 2)
	this.unsuback = newAckqueue(defaultQueueSize << 2)
	this.pingack = newAckqueue(defaultQueueSize << 2)

	for i := 0; i < len(topics); i++ {
		this.topics[topics[i].Topic] = topics[i].Qos
	}

	this.initted = true

	return nil
}

func (this *session) Update(msg *messagev5.ConnectMessage) error {
	this.mu.Lock()
	defer this.mu.Unlock()

	this.cbuf = make([]byte, msg.Len())
	this.cmsg = messagev5.NewConnectMessage()

	if _, err := msg.Encode(this.cbuf); err != nil {
		return err
	}

	if _, err := this.cmsg.Decode(this.cbuf); err != nil {
		return err
	}

	return nil
}

func (this *session) AddTopic(topic string, qos byte) error {
	this.mu.Lock()
	defer this.mu.Unlock()

	if !this.initted {
		return fmt.Errorf("session not yet initialized")
	}

	this.topics[topic] = qos

	return nil
}

func (this *session) RemoveTopic(topic string) error {
	this.mu.Lock()
	defer this.mu.Unlock()

	if !this.initted {
		return fmt.Errorf("session not yet initialized")
	}

	delete(this.topics, topic)

	return nil
}

func (this *session) Topics() ([]string, []byte, error) {
	this.mu.Lock()
	defer this.mu.Unlock()

	if !this.initted {
		return nil, nil, fmt.Errorf("session not yet initialized")
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

func (this *session) ID() string {
	return string(this.Cmsg().ClientId())
}
func (this *session) IDs() []byte {
	return this.Cmsg().ClientId()
}

func (this *session) Cmsg() *messagev5.ConnectMessage {
	return this.cmsg
}

func (this *session) Will() *messagev5.PublishMessage {
	return this.will
}

func (this *session) Pub1ack() Ackqueue {
	return this.pub1ack
}

func (this *session) Pub2in() Ackqueue {
	return this.pub2in
}

func (this *session) Pub2out() Ackqueue {
	return this.pub2out
}

func (this *session) Suback() Ackqueue {
	return this.suback
}

func (this *session) Unsuback() Ackqueue {
	return this.unsuback
}

func (this *session) Pingack() Ackqueue {
	return this.pingack
}

type SessionInitTopic struct {
	Topic string
	Qos   byte
}
type Session interface {
	Init(msg *messagev5.ConnectMessage, topics ...SessionInitTopic) error
	Update(msg *messagev5.ConnectMessage) error

	AddTopic(topic string, qos byte) error
	RemoveTopic(topic string) error
	Topics() ([]string, []byte, error)

	ID() string
	IDs() []byte

	Cmsg() *messagev5.ConnectMessage
	Will() *messagev5.PublishMessage

	Pub1ack() Ackqueue
	Pub2in() Ackqueue
	Pub2out() Ackqueue
	Suback() Ackqueue
	Unsuback() Ackqueue
	Pingack() Ackqueue
}
