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

	// cbuf is the CONNECT messagev5 buffer, sess is for storing all the will stuff
	//cbuf是连接消息缓冲区，用于存储所有的will内容
	cbuf []byte

	// rbuf is the retained PUBLISH messagev5 buffer
	// rbuf是保留的发布消息缓冲区
	rbuf []byte

	// topics stores all the topis for sess session/client
	//主题存储此会话/客户机的所有topics
	topics map[string]*topics.Sub

	topicAlice   map[uint16][]byte
	topicAliceRe map[string]uint16

	// Initialized?
	initted bool

	// Serialize access to sess session
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
func (sess *session) InitSample(msg *message.ConnectMessage, sessionStore store.SessionStore, tps ...sessions.SessionInitTopic) error {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	if sess.initted {
		return fmt.Errorf("session already initialized")
	}
	sess.cbuf = make([]byte, msg.Len())

	if _, err := msg.Encode(sess.cbuf); err != nil {
		return err
	}

	if _, err := sess.cmessage.Decode(sess.cbuf); err != nil {
		return err
	}

	if sess.cmessage.WillFlag() {
		sess.will = message.NewPublishMessage()
		sess.will.SetQoS(sess.cmessage.WillQos())
		sess.will.SetTopic(sess.cmessage.WillTopic())
		sess.will.SetPayload(sess.cmessage.WillMessage())
		sess.will.SetRetain(sess.cmessage.WillRetain())
	}

	sess.topics = make(map[string]*topics.Sub)
	sess.topicAlice = make(map[uint16][]byte)
	sess.topicAliceRe = make(map[string]uint16)

	sess.id = string(msg.ClientId())
	sess.pub1ack = newDbAckQueue(sessionStore, defaultQueueSize<<1, sess.id, false, true)
	sess.pub2in = newDbAckQueue(sessionStore, defaultQueueSize<<1, sess.id, true, false)
	sess.pub2out = newDbAckQueue(sessionStore, defaultQueueSize<<1, sess.id, false, true)
	sess.suback = newDbAckQueue(sessionStore, defaultQueueSize>>4, sess.id, false, false)
	sess.unsuback = newDbAckQueue(sessionStore, defaultQueueSize>>4, sess.id, false, false)
	sess.pingack = newDbAckQueue(sessionStore, defaultQueueSize>>4, sess.id, false, false)
	sess.runBatch()

	for i := 0; i < len(tps); i++ {
		sb := topics.Sub(tps[i])
		sess.topics[string(tps[i].Topic)] = &sb
	}

	sess.status = sessions.ONLINE

	sess.initted = true

	return nil
}
func (sess *session) runBatch() {
	go func() {
		b2o := sess.pub2out.(batchOption)
		b1o := sess.pub1ack.(batchOption)
		tg := 0
		for {
			if sess.stop == 2 {
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
func (sess *session) Init(msg *message.ConnectMessage, tps ...sessions.SessionInitTopic) error {
	sess.mu.Lock()
	defer sess.mu.Unlock()

	if sess.initted {
		return fmt.Errorf("session already initialized")
	}

	sess.cbuf = make([]byte, msg.Len())

	if _, err := msg.Encode(sess.cbuf); err != nil {
		return err
	}

	if _, err := sess.cmessage.Decode(sess.cbuf); err != nil {
		return err
	}

	if sess.cmessage.WillFlag() {
		sess.will = message.NewPublishMessage()
		sess.will.SetQoS(sess.cmessage.WillQos())
		sess.will.SetTopic(sess.cmessage.WillTopic())
		sess.will.SetPayload(sess.cmessage.WillMessage())
		sess.will.SetRetain(sess.cmessage.WillRetain())
	}

	sess.topics = make(map[string]*topics.Sub)
	sess.topicAlice = make(map[uint16][]byte)
	sess.topicAliceRe = make(map[string]uint16)

	sess.id = string(msg.ClientId())

	sess.pub1ack = newAckqueue(defaultQueueSize << 1)
	sess.pub2in = newAckqueue(defaultQueueSize << 1)
	sess.pub2out = newAckqueue(defaultQueueSize << 1)
	sess.suback = newAckqueue(defaultQueueSize >> 4)
	sess.unsuback = newAckqueue(defaultQueueSize >> 4)
	sess.pingack = newAckqueue(defaultQueueSize >> 4)

	for i := 0; i < len(tps); i++ {
		sb := topics.Sub(tps[i])
		sess.topics[string(tps[i].Topic)] = &sb
	}

	sess.status = sessions.ONLINE

	sess.initted = true

	return nil
}
func (sess *session) OfflineMsg() []message.Message {
	return nil
}
func (sess *session) Update(msg *message.ConnectMessage) error {
	sess.mu.Lock()
	defer sess.mu.Unlock()

	sess.cbuf = make([]byte, msg.Len())
	sess.cmessage = message.NewConnectMessage()

	if _, err := msg.Encode(sess.cbuf); err != nil {
		return err
	}

	if _, err := sess.cmessage.Decode(sess.cbuf); err != nil {
		return err
	}
	sess.stop = 1
	if sess.pingack != nil {
		if _, ok := sess.pingack.(*dbAckqueue); ok {
			sess.runBatch()
		}
	}
	return nil
}

func (sess *session) AddTopicAlice(topic []byte, alice uint16) {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	sess.topicAlice[alice] = topic
	sess.topicAliceRe[string(topic)] = alice
}
func (sess *session) GetTopicAlice(topic []byte) (uint16, bool) {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	tp, exist := sess.topicAliceRe[string(topic)]
	return tp, exist
}
func (sess *session) GetAliceTopic(alice uint16) ([]byte, bool) {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	tp, exist := sess.topicAlice[alice]
	return tp, exist
}
func (sess *session) AddTopic(sub topics.Sub) error {
	sess.mu.Lock()
	defer sess.mu.Unlock()

	if !sess.initted {
		return fmt.Errorf("session not yet initialized")
	}

	sess.topics[string(sub.Topic)] = &sub

	return nil
}

func (sess *session) RemoveTopic(topic string) error {
	sess.mu.Lock()
	defer sess.mu.Unlock()

	if !sess.initted {
		return fmt.Errorf("session not yet initialized")
	}

	delete(sess.topics, topic)

	return nil
}
func (sess *session) SubOption(topic []byte) topics.Sub {
	sess.mu.Lock()
	defer sess.mu.Unlock()
	sub, ok := sess.topics[string(topic)]
	if !ok {
		return topics.Sub{}
	}
	return *sub
}
func (sess *session) Topics() ([]topics.Sub, error) {
	sess.mu.Lock()
	defer sess.mu.Unlock()

	if !sess.initted {
		return nil, fmt.Errorf("session not yet initialized")
	}

	var (
		subs []topics.Sub
	)

	for _, v := range sess.topics {
		subs = append(subs, *v)
	}

	return subs, nil
}

func (sess *session) ID() string {
	return string(sess.Cmsg().ClientId())
}
func (sess *session) IDs() []byte {
	return sess.Cmsg().ClientId()
}

func (sess *session) Cmsg() *message.ConnectMessage {
	return sess.cmessage
}

func (sess *session) Will() *message.PublishMessage {
	if sess.stop == 1 {
		sess.stop = 2
	} else {
		sess.stop = 1
	}
	return sess.will
}

func (sess *session) Pub1ack() sessions.Ackqueue {
	return sess.pub1ack
}

func (sess *session) Pub2in() sessions.Ackqueue {
	return sess.pub2in
}

func (sess *session) Pub2out() sessions.Ackqueue {
	return sess.pub2out
}

func (sess *session) Suback() sessions.Ackqueue {
	return sess.suback
}

func (sess *session) Unsuback() sessions.Ackqueue {
	return sess.unsuback
}

func (sess *session) Pingack() sessions.Ackqueue {
	return sess.pingack
}

func (sess *session) ExpiryInterval() uint32 {
	return sess.cmessage.SessionExpiryInterval()
}

func (sess *session) Status() sessions.Status {
	return sess.status
}

func (sess *session) ReceiveMaximum() uint16 {
	return sess.cmessage.ReceiveMaximum()
}

func (sess *session) MaxPacketSize() uint32 {
	return sess.cmessage.MaxPacketSize()
}

func (sess *session) TopicAliasMax() uint16 {
	return sess.cmessage.TopicAliasMax()
}

func (sess *session) RequestRespInfo() byte {
	return sess.cmessage.RequestRespInfo()
}

func (sess *session) RequestProblemInfo() byte {
	return sess.cmessage.RequestProblemInfo()
}

func (sess *session) UserProperty() []string {
	u := sess.cmessage.UserProperty()
	up := make([]string, len(u))
	for i := 0; i < len(u); i++ {
		up[i] = string(u[i])
	}
	return up
}

func (sess *session) OfflineTime() int64 {
	return sess.offlineTime
}

func (sess *session) ClientId() string {
	return string(sess.cmessage.ClientId())
}

func (sess *session) SetClientId(s string) {
	_ = sess.cmessage.SetClientId([]byte(s))
}

func (sess *session) SetExpiryInterval(u uint32) {
	sess.cmessage.SetSessionExpiryInterval(u)
}

func (sess *session) SetStatus(status sessions.Status) {
	sess.status = status
}

func (sess *session) SetReceiveMaximum(u uint16) {
	sess.cmessage.SetReceiveMaximum(u)
}

func (sess *session) SetMaxPacketSize(u uint32) {
	sess.cmessage.SetMaxPacketSize(u)
}

func (sess *session) SetTopicAliasMax(u uint16) {
	sess.cmessage.SetTopicAliasMax(u)
}

func (sess *session) SetRequestRespInfo(b byte) {
	sess.cmessage.SetRequestRespInfo(b)
}

func (sess *session) SetRequestProblemInfo(b byte) {
	sess.cmessage.SetRequestProblemInfo(b)
}

func (sess *session) SetUserProperty(up []string) {
	u := make([][]byte, len(up))
	for i := 0; i < len(up); i++ {
		u[i] = []byte(up[i])
	}
	sess.cmessage.AddUserPropertys(u)
}

func (sess *session) SetOfflineTime(i int64) {
	sess.offlineTime = i
}

func (sess *session) SetWill(will *message.PublishMessage) {
	sess.will = will
}

func (sess *session) SetSub(sub *message.SubscribeMessage) {
	tp := sub.Topics()
	qos := sub.Qos()
	for i := 0; i < len(tp); i++ {
		_ = sess.AddTopic(topics.Sub{
			Topic:             tp[i],
			Qos:               qos[i],
			NoLocal:           sub.TopicNoLocal(tp[i]),
			RetainAsPublished: sub.TopicRetainAsPublished(tp[i]),
			RetainHandling:    sub.TopicRetainHandling(tp[i]),
			SubIdentifier:     sub.SubscriptionIdentifier(),
		})
	}
}
func (sess *session) SetStore(_ store.SessionStore, _ store.MessageStore) {
}
