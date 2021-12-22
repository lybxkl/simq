package impl

import (
	"context"
	"gitee.com/Ljolan/si-mqtt/cluster/store"
	messagev2 "gitee.com/Ljolan/si-mqtt/corev5/v2/message"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/sessions"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/topics"
	"gitee.com/Ljolan/si-mqtt/logger"
	"time"
)

type dbSession struct {
	sessionStore store.SessionStore
	messageStore store.MessageStore
	memSession   *session
	offline      []messagev2.Message
}

func NewDBSession(id string) sessions.Session {
	return &dbSession{memSession: NewMemSession(id)}
}
func NewDBSessionSample(sessionStore store.SessionStore,
	messageStore store.MessageStore) sessions.Session {
	return &dbSession{sessionStore: sessionStore, messageStore: messageStore}
}

func (d *dbSession) SetStore(sessionStore store.SessionStore, messageStore store.MessageStore) {
	d.sessionStore = sessionStore
	d.messageStore = messageStore
}

// 此dbsession的init不会有topic数据来
func (d *dbSession) Init(msg *messagev2.ConnectMessage, _ ...sessions.SessionInitTopic) error {
	cid := string(msg.ClientId())
	ctx := context.Background()
	// 拉取订阅
	subs, err := d.sessionStore.GetSubscriptions(ctx, cid)
	tps := make([]sessions.SessionInitTopic, len(subs))
	for i := 0; i < len(subs); i++ {
		topic := subs[i].Topics()
		if len(topic) == 0 {
			continue
		}
		tpc := topic[0] // 正常来说 从数据库取出来的 只会有一个topic
		tps[i] = sessions.SessionInitTopic{
			Topic:             tpc,
			Qos:               subs[i].Qos()[0],
			NoLocal:           subs[i].TopicNoLocal(tpc),
			RetainAsPublished: subs[i].TopicRetainAsPublished(tpc),
			RetainHandling:    subs[i].TopicRetainHandling(tpc),
			SubIdentifier:     subs[i].SubscriptionIdentifier(),
		}
	}
	err = d.memSession.InitSample(msg, d.sessionStore, tps...)
	if err != nil {
		return err
	}
	// 拉取过程消息，inflow，outflow，outflow2，离线消息
	// info
	info, err := d.sessionStore.GetAllInflowMsg(ctx, cid)
	if err != nil {
		logger.Logger.Errorf("get client: %v all info message error: %v", cid, err)
	}
	for i := 0; i < len(info); i++ {
		// 如果超过int64一半 会导致panic
		_ = d.memSession.pub2in.Wait(info[i], func(msg, ack messagev2.Message, err error) {
			if err != nil {
				logger.Logger.Debugf("发送成功：%v,%v,%v", msg, ack, err)
			} else {
				logger.Logger.Debugf("发送失败：%v,%v,%v", msg, ack, err)
			}
		})
	}
	// outflow
	outflow, err := d.sessionStore.GetAllOutflowMsg(ctx, cid)
	if err != nil {
		logger.Logger.Errorf("get client: %v all outflow message error: %v", cid, err)
	}
	for i := 0; i < len(outflow); i++ {
		if of := outflow[i].(*messagev2.PublishMessage); of.QoS() == 2 {
			// 如果超过int64一半 会导致panic
			_ = d.memSession.pub2out.Wait(of, func(msg, ack messagev2.Message, err error) {
				if err != nil {
					logger.Logger.Debugf("发送成功：%v,%v,%v", msg, ack, err)
				} else {
					logger.Logger.Debugf("发送失败：%v,%v,%v", msg, ack, err)
				}
			})
		} else {
			// 如果超过int64一半 会导致panic
			_ = d.memSession.pub1ack.Wait(of, func(msg, ack messagev2.Message, err error) {
				if err != nil {
					logger.Logger.Debugf("发送成功：%v,%v,%v", msg, ack, err)
				} else {
					logger.Logger.Debugf("发送失败：%v,%v,%v", msg, ack, err)
				}
			})
		}
	}
	// outflow2
	outflow2, err := d.sessionStore.GetAllOutflowSecMsg(ctx, cid)
	if err != nil {
		logger.Logger.Errorf("get client: %v all outflow2 message error: %v", cid, err)
	}
	for i := 0; i < len(outflow2); i++ {
		outflow2ack := messagev2.NewPublishMessage()
		outflow2ack.SetPacketId(outflow2[i])
		// 如果超过int64一半 会导致panic
		_ = d.memSession.pub2out.Wait(outflow2ack, func(msg, ack messagev2.Message, err error) {
			if err != nil {
				logger.Logger.Debugf("发送成功：%v,%v,%v", msg, ack, err)
			} else {
				logger.Logger.Debugf("发送失败：%v,%v,%v", msg, ack, err)
			}
		})
	}
	// 将离线消息qos>0的转至outflow表中，再删除离线消息表中数据
	offline, _, err := d.sessionStore.GetAllOfflineMsg(ctx, cid)
	if err != nil {
		logger.Logger.Errorf("get client: %v all offline message error: %v", cid, err)
	}
	if len(offline) > 0 {
		err = d.sessionStore.ClearOfflineMsgs(ctx, cid) // fixme 提前删除不太好
		if err != nil {
			logger.Logger.Errorf("del client: %v all offline message error: %v", cid, err)
		}
		d.offline = offline
	}
	err = d.sessionStore.StoreSession(ctx, cid, d.memSession)
	if err != nil {
		return err
	}

	return nil
}
func (d *dbSession) OfflineMsg() []messagev2.Message {
	return d.offline
}
func (d *dbSession) Update(msg *messagev2.ConnectMessage) error {
	cid := string(msg.ClientId())
	ctx := context.Background()
	err := d.sessionStore.StoreSession(ctx, cid, NewMemSessionByCon(msg))
	if err != nil {
		return err
	}
	_ = d.memSession.Update(msg)
	return nil
}
func (this *dbSession) AddTopicAlice(topic []byte, alice uint16) {
	this.memSession.AddTopicAlice(topic, alice)
}
func (this *dbSession) GetAliceTopic(alice uint16) ([]byte, bool) {
	return this.memSession.GetAliceTopic(alice)
}
func (this *dbSession) GetTopicAlice(topic []byte) (uint16, bool) {
	return this.memSession.GetTopicAlice(topic)
}
func (d *dbSession) AddTopic(subs topics.Sub) error {
	sub := messagev2.NewSubscribeMessage()
	err := sub.AddTopicAll(subs.Topic, subs.Qos, subs.NoLocal, subs.RetainAsPublished, byte(subs.RetainHandling))
	if err != nil {
		return err
	}
	sub.SetSubscriptionIdentifier(subs.SubIdentifier)
	ctx := context.Background()
	err = d.sessionStore.StoreSubscription(ctx, d.memSession.ClientId(), sub)
	if err != nil {
		return err
	}
	_ = d.memSession.AddTopic(subs)
	return nil
}

func (d *dbSession) RemoveTopic(topic string) error {
	ctx := context.Background()
	err := d.sessionStore.DelSubscription(ctx, d.memSession.ClientId(), topic)
	if err != nil {
		return err
	}
	_ = d.memSession.RemoveTopic(topic)
	return nil
}

func (d *dbSession) Topics() ([]topics.Sub, error) {
	return d.memSession.Topics()
}
func (d *dbSession) SubOption(topic []byte) topics.Sub {
	return d.memSession.SubOption(topic)
}
func (d *dbSession) ID() string {
	return d.memSession.ID()
}

func (d *dbSession) IDs() []byte {
	return d.memSession.IDs()
}

func (d *dbSession) Cmsg() *messagev2.ConnectMessage {
	return d.memSession.Cmsg()
}

func (d *dbSession) Will() *messagev2.PublishMessage {
	return d.memSession.Will()
}

func (d *dbSession) Pub1ack() sessions.Ackqueue {
	return d.memSession.Pub1ack()
}

func (d *dbSession) Pub2in() sessions.Ackqueue {
	return d.memSession.Pub2in()
}

func (d *dbSession) Pub2out() sessions.Ackqueue {
	return d.memSession.Pub2out()
}

func (d *dbSession) Suback() sessions.Ackqueue {
	return d.memSession.Suback()
}

func (d *dbSession) Unsuback() sessions.Ackqueue {
	return d.memSession.Unsuback()
}

func (d *dbSession) Pingack() sessions.Ackqueue {
	return d.memSession.Pingack()
}

func (d *dbSession) ExpiryInterval() uint32 {
	return d.memSession.ExpiryInterval()
}

func (d *dbSession) Status() sessions.Status {
	return d.memSession.Status()
}

func (d *dbSession) ClientId() string {
	return d.memSession.ClientId()
}

func (d *dbSession) ReceiveMaximum() uint16 {
	return d.memSession.ReceiveMaximum()
}

func (d *dbSession) MaxPacketSize() uint32 {
	return d.memSession.MaxPacketSize()
}

func (d *dbSession) TopicAliasMax() uint16 {
	return d.memSession.TopicAliasMax()
}

func (d *dbSession) RequestRespInfo() byte {
	return d.memSession.RequestRespInfo()
}

func (d *dbSession) RequestProblemInfo() byte {
	return d.memSession.RequestProblemInfo()
}

func (d *dbSession) UserProperty() []string {
	return d.memSession.UserProperty()
}

func (d *dbSession) OfflineTime() int64 {
	return d.memSession.OfflineTime()
}

func (d *dbSession) SetExpiryInterval(u uint32) {
	d.memSession.SetExpiryInterval(u)
}

func (d *dbSession) SetStatus(status sessions.Status) {
	d.memSession.SetStatus(status)
	d.memSession.SetOfflineTime(time.Now().UnixNano())
	_ = d.sessionStore.StoreSession(context.Background(), d.ClientId(), d)
}

func (d *dbSession) SetClientId(s string) {
	d.memSession.SetClientId(s)
}

func (d *dbSession) SetReceiveMaximum(u uint16) {
	d.memSession.SetReceiveMaximum(u)
}

func (d *dbSession) SetMaxPacketSize(u uint32) {
	d.memSession.SetMaxPacketSize(u)
}

func (d *dbSession) SetTopicAliasMax(u uint16) {
	d.memSession.SetTopicAliasMax(u)
}

func (d *dbSession) SetRequestRespInfo(b byte) {
	d.memSession.SetRequestRespInfo(b)
}

func (d *dbSession) SetRequestProblemInfo(b byte) {
	d.memSession.SetRequestProblemInfo(b)
}

func (d *dbSession) SetUserProperty(up []string) {
	d.memSession.SetUserProperty(up)
}

func (d *dbSession) SetOfflineTime(i int64) {
	d.memSession.SetOfflineTime(i)
}
func (this *dbSession) SetWill(will *messagev2.PublishMessage) {
	this.memSession.SetWill(will)
}

func (this *dbSession) SetSub(sub *messagev2.SubscribeMessage) {
	this.memSession.SetSub(sub)
}
