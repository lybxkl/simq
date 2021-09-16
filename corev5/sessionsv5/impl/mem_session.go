package impl

import (
	"fmt"
	"gitee.com/Ljolan/si-mqtt/cluster/store"
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"gitee.com/Ljolan/si-mqtt/corev5/sessionsv5"
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
	pub1ack sessionsv5.Ackqueue

	// Ack queue for incoming PUBLISH QoS 2 messages
	//传入发布QoS 2消息的Ack队列
	pub2in sessionsv5.Ackqueue

	// Ack queue for outgoing PUBLISH QoS 2 messages
	//用于传出发布QoS 2消息的Ack队列
	pub2out sessionsv5.Ackqueue

	// Ack queue for outgoing SUBSCRIBE messages
	//用于发送订阅消息的Ack队列
	suback sessionsv5.Ackqueue

	// Ack queue for outgoing UNSUBSCRIBE messages
	//发送取消订阅消息的Ack队列
	unsuback sessionsv5.Ackqueue

	// Ack queue for outgoing PINGREQ messages
	//用于发送PINGREQ消息的Ack队列
	pingack sessionsv5.Ackqueue

	// cmsg is the CONNECT messagev5
	//cmsg是连接消息
	cmsg        *messagev5.ConnectMessage
	status      sessionsv5.Status // session状态
	offlineTime int64             // 离线时间

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

func NewMemSessionSampl() sessionsv5.Session {
	return &session{cmsg: messagev5.NewConnectMessage()}
}
func NewMemSession(id string) *session {
	return &session{id: id, cmsg: messagev5.NewConnectMessage()}
}
func NewMemSessionByCon(con *messagev5.ConnectMessage) *session {
	return &session{cmsg: con}
}
func (this *session) InitSample(msg *messagev5.ConnectMessage, sessionStore store.SessionStore, topics ...sessionsv5.SessionInitTopic) error {
	this.mu.Lock()
	defer this.mu.Unlock()
	if this.initted {
		return fmt.Errorf("session already initialized")
	}
	this.cbuf = make([]byte, msg.Len())

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
	this.pub1ack = newDbAckQueue(sessionStore, defaultQueueSize<<1, this.id, false)
	this.pub2in = newDbAckQueue(sessionStore, defaultQueueSize<<1, this.id, true)
	this.pub2out = newDbAckQueue(sessionStore, defaultQueueSize<<1, this.id, false)
	this.suback = newDbAckQueue(sessionStore, defaultQueueSize>>4, this.id, false)
	this.unsuback = newDbAckQueue(sessionStore, defaultQueueSize>>4, this.id, false)
	this.pingack = newDbAckQueue(sessionStore, defaultQueueSize>>4, this.id, false)

	for i := 0; i < len(topics); i++ {
		this.topics[topics[i].Topic] = topics[i].Qos
	}

	this.status = sessionsv5.ONLINE

	this.initted = true

	return nil
}

//Init 遗嘱和connect消息会存在每个session中，不用每次查询数据库的
func (this *session) Init(msg *messagev5.ConnectMessage, topics ...sessionsv5.SessionInitTopic) error {
	this.mu.Lock()
	defer this.mu.Unlock()

	if this.initted {
		return fmt.Errorf("session already initialized")
	}

	this.cbuf = make([]byte, msg.Len())

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

	this.pub1ack = newAckqueue(defaultQueueSize << 1)
	this.pub2in = newAckqueue(defaultQueueSize << 1)
	this.pub2out = newAckqueue(defaultQueueSize << 1)
	this.suback = newAckqueue(defaultQueueSize >> 4)
	this.unsuback = newAckqueue(defaultQueueSize >> 4)
	this.pingack = newAckqueue(defaultQueueSize >> 4)

	for i := 0; i < len(topics); i++ {
		this.topics[topics[i].Topic] = topics[i].Qos
	}

	this.status = sessionsv5.ONLINE

	this.initted = true

	return nil
}
func (this *session) OfflineMsg() []messagev5.Message {
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

func (this *session) Pub1ack() sessionsv5.Ackqueue {
	return this.pub1ack
}

func (this *session) Pub2in() sessionsv5.Ackqueue {
	return this.pub2in
}

func (this *session) Pub2out() sessionsv5.Ackqueue {
	return this.pub2out
}

func (this *session) Suback() sessionsv5.Ackqueue {
	return this.suback
}

func (this *session) Unsuback() sessionsv5.Ackqueue {
	return this.unsuback
}

func (this *session) Pingack() sessionsv5.Ackqueue {
	return this.pingack
}

func (this *session) ExpiryInterval() uint32 {
	return this.cmsg.SessionExpiryInterval()
}

func (this *session) Status() sessionsv5.Status {
	return this.status
}

func (this *session) ReceiveMaximum() uint16 {
	return this.cmsg.ReceiveMaximum()
}

func (this *session) MaxPacketSize() uint32 {
	return this.cmsg.MaxPacketSize()
}

func (this *session) TopicAliasMax() uint16 {
	return this.cmsg.TopicAliasMax()
}

func (this *session) RequestRespInfo() byte {
	return this.cmsg.RequestRespInfo()
}

func (this *session) RequestProblemInfo() byte {
	return this.cmsg.RequestProblemInfo()
}

func (this *session) UserProperty() []string {
	u := this.cmsg.UserProperty()
	up := make([]string, len(u))
	for i := 0; i < len(u); i++ {
		up[i] = string(u[i])
	}
	return up
}

func (this *session) OfflineTime() int64 {
	return this.offlineTime
}

func (this *session) ClientId() string {
	return string(this.cmsg.ClientId())
}

func (this *session) SetClientId(s string) {
	_ = this.cmsg.SetClientId([]byte(s))
}

func (this *session) SetExpiryInterval(u uint32) {
	this.cmsg.SetSessionExpiryInterval(u)
}

func (this *session) SetStatus(status sessionsv5.Status) {
	this.status = status
}

func (this *session) SetReceiveMaximum(u uint16) {
	this.cmsg.SetReceiveMaximum(u)
}

func (this *session) SetMaxPacketSize(u uint32) {
	this.cmsg.SetMaxPacketSize(u)
}

func (this *session) SetTopicAliasMax(u uint16) {
	this.cmsg.SetTopicAliasMax(u)
}

func (this *session) SetRequestRespInfo(b byte) {
	this.cmsg.SetRequestRespInfo(b)
}

func (this *session) SetRequestProblemInfo(b byte) {
	this.cmsg.SetRequestProblemInfo(b)
}

func (this *session) SetUserProperty(up []string) {
	u := make([][]byte, len(up))
	for i := 0; i < len(up); i++ {
		u[i] = []byte(up[i])
	}
	this.cmsg.AddUserPropertys(u)
}

func (this *session) SetOfflineTime(i int64) {
	this.offlineTime = i
}

func (this *session) SetWill(will *messagev5.PublishMessage) {
	this.will = will
}

func (this *session) SetSub(sub *messagev5.SubscribeMessage) {
	tp := sub.Topics()
	qos := sub.Qos()
	for i := 0; i < len(tp); i++ {
		_ = this.AddTopic(string(tp[i]), qos[i])
	}
}
func (this *session) SetStore(_ store.SessionStore, _ store.MessageStore) {
}
