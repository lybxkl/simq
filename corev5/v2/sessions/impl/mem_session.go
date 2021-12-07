package impl

import (
	"fmt"
	"gitee.com/Ljolan/si-mqtt/cluster/store"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/message"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/sessions"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/topics"
	"sync"
	"time"
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
	pub1ack sessions.Ackqueue

	// Ack queue for incoming PUBLISH QoS 2 messages
	//传入发布QoS 2消息的Ack队列
	pub2in sessions.Ackqueue

	// Ack queue for outgoing PUBLISH QoS 2 messages
	//用于传出发布QoS 2消息的Ack队列
	pub2out sessions.Ackqueue

	// Ack queue for outgoing SUBSCRIBE messages
	//用于发送订阅消息的Ack队列
	suback sessions.Ackqueue

	// Ack queue for outgoing UNSUBSCRIBE messages
	//发送取消订阅消息的Ack队列
	unsuback sessions.Ackqueue

	// Ack queue for outgoing PINGREQ messages
	//用于发送PINGREQ消息的Ack队列
	pingack sessions.Ackqueue

	// cmessage is the CONNECT messagev5
	//cmessage是连接消息
	cmessage    *message.ConnectMessage
	status      sessions.Status // session状态
	offlineTime int64           // 离线时间

	// Will messagev5 to publish if connect is closed unexpectedly
	//如果连接意外关闭，遗嘱消息将发布
	will *message.PublishMessage

	// cbuf is the CONNECT messagev5 buffer, this is for storing all the will stuff
	//cbuf是连接消息缓冲区，用于存储所有的will内容
	cbuf []byte

	// rbuf is the retained PUBLISH messagev5 buffer
	// rbuf是保留的发布消息缓冲区
	rbuf []byte

	// topics stores all the topis for this session/client
	//主题存储此会话/客户机的所有topics
	topics map[string]*topics.Sub

	topicAlice   map[uint16][]byte
	topicAliceRe map[string]uint16

	// Initialized?
	initted bool

	// Serialize access to this session
	//序列化对该会话的访问锁
	mu   sync.Mutex
	stop int8 // 2 为关闭
	id   string
}

func NewMemSession(id string) *session {
	return &session{id: id, cmessage: message.NewConnectMessage()}
}
func NewMemSessionByCon(con *message.ConnectMessage) *session {
	return &session{cmessage: con}
}
func (this *session) InitSample(msg *message.ConnectMessage, sessionStore store.SessionStore, tps ...sessions.SessionInitTopic) error {
	this.mu.Lock()
	defer this.mu.Unlock()
	if this.initted {
		return fmt.Errorf("session already initialized")
	}
	this.cbuf = make([]byte, msg.Len())

	if _, err := msg.Encode(this.cbuf); err != nil {
		return err
	}

	if _, err := this.cmessage.Decode(this.cbuf); err != nil {
		return err
	}

	if this.cmessage.WillFlag() {
		this.will = message.NewPublishMessage()
		this.will.SetQoS(this.cmessage.WillQos())
		this.will.SetTopic(this.cmessage.WillTopic())
		this.will.SetPayload(this.cmessage.WillMessage())
		this.will.SetRetain(this.cmessage.WillRetain())
	}

	this.topics = make(map[string]*topics.Sub)
	this.topicAlice = make(map[uint16][]byte)
	this.topicAliceRe = make(map[string]uint16)

	this.id = string(msg.ClientId())
	this.pub1ack = newDbAckQueue(sessionStore, defaultQueueSize<<1, this.id, false, true)
	this.pub2in = newDbAckQueue(sessionStore, defaultQueueSize<<1, this.id, true, false)
	this.pub2out = newDbAckQueue(sessionStore, defaultQueueSize<<1, this.id, false, true)
	this.suback = newDbAckQueue(sessionStore, defaultQueueSize>>4, this.id, false, false)
	this.unsuback = newDbAckQueue(sessionStore, defaultQueueSize>>4, this.id, false, false)
	this.pingack = newDbAckQueue(sessionStore, defaultQueueSize>>4, this.id, false, false)
	this.runBatch()

	for i := 0; i < len(tps); i++ {
		sb := topics.Sub(tps[i])
		this.topics[string(tps[i].Topic)] = &sb
	}

	this.status = sessions.ONLINE

	this.initted = true

	return nil
}
func (this *session) runBatch() {
	go func() {
		b2o := this.pub2out.(batchOption)
		b1o := this.pub1ack.(batchOption)
		tg := 0
		for {
			if this.stop == 2 {
				return
			}
			select {
			case <-time.After(10 * time.Millisecond):
				if b2o.GetNum() > 0 {
					_ = b2o.BatchReleaseOutflowSecMsgId()
				} else {
					tg++
				}
				if b1o.GetNum() > 0 {
					_ = b1o.BatchReleaseOutflowMsg()
				} else {
					tg++
				}
				if tg == 2 {
					tg = 0
					time.Sleep(5 * time.Millisecond)
				}
			}
		}
	}()
}

//Init 遗嘱和connect消息会存在每个session中，不用每次查询数据库的
func (this *session) Init(msg *message.ConnectMessage, tps ...sessions.SessionInitTopic) error {
	this.mu.Lock()
	defer this.mu.Unlock()

	if this.initted {
		return fmt.Errorf("session already initialized")
	}

	this.cbuf = make([]byte, msg.Len())

	if _, err := msg.Encode(this.cbuf); err != nil {
		return err
	}

	if _, err := this.cmessage.Decode(this.cbuf); err != nil {
		return err
	}

	if this.cmessage.WillFlag() {
		this.will = message.NewPublishMessage()
		this.will.SetQoS(this.cmessage.WillQos())
		this.will.SetTopic(this.cmessage.WillTopic())
		this.will.SetPayload(this.cmessage.WillMessage())
		this.will.SetRetain(this.cmessage.WillRetain())
	}

	this.topics = make(map[string]*topics.Sub)
	this.topicAlice = make(map[uint16][]byte)
	this.topicAliceRe = make(map[string]uint16)

	this.id = string(msg.ClientId())

	this.pub1ack = newAckqueue(defaultQueueSize << 1)
	this.pub2in = newAckqueue(defaultQueueSize << 1)
	this.pub2out = newAckqueue(defaultQueueSize << 1)
	this.suback = newAckqueue(defaultQueueSize >> 4)
	this.unsuback = newAckqueue(defaultQueueSize >> 4)
	this.pingack = newAckqueue(defaultQueueSize >> 4)

	for i := 0; i < len(tps); i++ {
		sb := topics.Sub(tps[i])
		this.topics[string(tps[i].Topic)] = &sb
	}

	this.status = sessions.ONLINE

	this.initted = true

	return nil
}
func (this *session) OfflineMsg() []message.Message {
	return nil
}
func (this *session) Update(msg *message.ConnectMessage) error {
	this.mu.Lock()
	defer this.mu.Unlock()

	this.cbuf = make([]byte, msg.Len())
	this.cmessage = message.NewConnectMessage()

	if _, err := msg.Encode(this.cbuf); err != nil {
		return err
	}

	if _, err := this.cmessage.Decode(this.cbuf); err != nil {
		return err
	}
	this.stop = 1
	if this.pingack != nil {
		if _, ok := this.pingack.(*dbAckqueue); ok {
			this.runBatch()
		}
	}
	return nil
}

func (this *session) AddTopicAlice(topic []byte, alice uint16) {
	this.mu.Lock()
	defer this.mu.Unlock()
	this.topicAlice[alice] = topic
	this.topicAliceRe[string(topic)] = alice
}
func (this *session) GetTopicAlice(topic []byte) (uint16, bool) {
	this.mu.Lock()
	defer this.mu.Unlock()
	tp, exist := this.topicAliceRe[string(topic)]
	return tp, exist
}
func (this *session) GetAliceTopic(alice uint16) ([]byte, bool) {
	this.mu.Lock()
	defer this.mu.Unlock()
	tp, exist := this.topicAlice[alice]
	return tp, exist
}
func (this *session) AddTopic(sub topics.Sub) error {
	this.mu.Lock()
	defer this.mu.Unlock()

	if !this.initted {
		return fmt.Errorf("session not yet initialized")
	}

	this.topics[string(sub.Topic)] = &sub

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
func (this *session) SubOption(topic []byte) topics.Sub {
	this.mu.Lock()
	defer this.mu.Unlock()
	sub, ok := this.topics[string(topic)]
	if !ok {
		return topics.Sub{}
	}
	return *sub
}
func (this *session) Topics() ([]topics.Sub, error) {
	this.mu.Lock()
	defer this.mu.Unlock()

	if !this.initted {
		return nil, fmt.Errorf("session not yet initialized")
	}

	var (
		subs []topics.Sub
	)

	for _, v := range this.topics {
		subs = append(subs, *v)
	}

	return subs, nil
}

func (this *session) ID() string {
	return string(this.Cmsg().ClientId())
}
func (this *session) IDs() []byte {
	return this.Cmsg().ClientId()
}

func (this *session) Cmsg() *message.ConnectMessage {
	return this.cmessage
}

func (this *session) Will() *message.PublishMessage {
	if this.stop == 1 {
		this.stop = 2
	} else {
		this.stop = 1
	}
	return this.will
}

func (this *session) Pub1ack() sessions.Ackqueue {
	return this.pub1ack
}

func (this *session) Pub2in() sessions.Ackqueue {
	return this.pub2in
}

func (this *session) Pub2out() sessions.Ackqueue {
	return this.pub2out
}

func (this *session) Suback() sessions.Ackqueue {
	return this.suback
}

func (this *session) Unsuback() sessions.Ackqueue {
	return this.unsuback
}

func (this *session) Pingack() sessions.Ackqueue {
	return this.pingack
}

func (this *session) ExpiryInterval() uint32 {
	return this.cmessage.SessionExpiryInterval()
}

func (this *session) Status() sessions.Status {
	return this.status
}

func (this *session) ReceiveMaximum() uint16 {
	return this.cmessage.ReceiveMaximum()
}

func (this *session) MaxPacketSize() uint32 {
	return this.cmessage.MaxPacketSize()
}

func (this *session) TopicAliasMax() uint16 {
	return this.cmessage.TopicAliasMax()
}

func (this *session) RequestRespInfo() byte {
	return this.cmessage.RequestRespInfo()
}

func (this *session) RequestProblemInfo() byte {
	return this.cmessage.RequestProblemInfo()
}

func (this *session) UserProperty() []string {
	u := this.cmessage.UserProperty()
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
	return string(this.cmessage.ClientId())
}

func (this *session) SetClientId(s string) {
	_ = this.cmessage.SetClientId([]byte(s))
}

func (this *session) SetExpiryInterval(u uint32) {
	this.cmessage.SetSessionExpiryInterval(u)
}

func (this *session) SetStatus(status sessions.Status) {
	this.status = status
}

func (this *session) SetReceiveMaximum(u uint16) {
	this.cmessage.SetReceiveMaximum(u)
}

func (this *session) SetMaxPacketSize(u uint32) {
	this.cmessage.SetMaxPacketSize(u)
}

func (this *session) SetTopicAliasMax(u uint16) {
	this.cmessage.SetTopicAliasMax(u)
}

func (this *session) SetRequestRespInfo(b byte) {
	this.cmessage.SetRequestRespInfo(b)
}

func (this *session) SetRequestProblemInfo(b byte) {
	this.cmessage.SetRequestProblemInfo(b)
}

func (this *session) SetUserProperty(up []string) {
	u := make([][]byte, len(up))
	for i := 0; i < len(up); i++ {
		u[i] = []byte(up[i])
	}
	this.cmessage.AddUserPropertys(u)
}

func (this *session) SetOfflineTime(i int64) {
	this.offlineTime = i
}

func (this *session) SetWill(will *message.PublishMessage) {
	this.will = will
}

func (this *session) SetSub(sub *message.SubscribeMessage) {
	tp := sub.Topics()
	qos := sub.Qos()
	for i := 0; i < len(tp); i++ {
		_ = this.AddTopic(topics.Sub{
			Topic:             tp[i],
			Qos:               qos[i],
			NoLocal:           sub.TopicNoLocal(tp[i]),
			RetainAsPublished: sub.TopicRetainAsPublished(tp[i]),
			RetainHandling:    sub.TopicRetainHandling(tp[i]),
			SubIdentifier:     sub.SubscriptionIdentifier(),
		})
	}
}
func (this *session) SetStore(_ store.SessionStore, _ store.MessageStore) {
}
