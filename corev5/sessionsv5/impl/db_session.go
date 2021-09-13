package impl

import (
	"context"
	"gitee.com/Ljolan/si-mqtt/cluster/store"
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"gitee.com/Ljolan/si-mqtt/corev5/sessionsv5"
)

type dbSession struct {
	sessionStore store.SessionStore
	messageStore store.MessageStore
	memSession   *session
}

func NewDBSession(id string) sessionsv5.Session {
	return &dbSession{memSession: NewMemSession(id)}
}
func NewDBSessionSample(sessionStore store.SessionStore,
	messageStore store.MessageStore) sessionsv5.Session {
	return &dbSession{sessionStore: sessionStore, messageStore: messageStore}
}

func (d *dbSession) SetStore(sessionStore store.SessionStore, messageStore store.MessageStore) {
	d.sessionStore = sessionStore
	d.messageStore = messageStore
}

func (d *dbSession) Init(msg *messagev5.ConnectMessage, topics ...sessionsv5.SessionInitTopic) error {
	cid := string(msg.ClientId())
	ctx := context.Background()

	err := d.memSession.InitSample(msg, d.sessionStore, topics...)
	if err != nil {
		return err
	}
	err = d.sessionStore.StoreSession(ctx, cid, d.memSession)
	if err != nil {
		return err
	}
	if len(topics) == 0 {
		return nil
	}
	sub := messagev5.NewSubscribeMessage()
	for i := 0; i < len(topics); i++ {
		err = sub.AddTopic([]byte(topics[i].Topic), topics[i].Qos)
		if err != nil {
			return err
		}
	}
	return d.sessionStore.StoreSubscription(ctx, cid, sub)
}

func (d *dbSession) Update(msg *messagev5.ConnectMessage) error {
	cid := string(msg.ClientId())
	ctx := context.Background()
	err := d.sessionStore.StoreSession(ctx, cid, NewMemSessionByCon(msg))
	if err != nil {
		return err
	}
	_ = d.memSession.Update(msg)
	return nil
}

func (d *dbSession) AddTopic(topic string, qos byte) error {
	sub := messagev5.NewSubscribeMessage()
	err := sub.AddTopic([]byte(topic), qos)
	if err != nil {
		return err
	}
	ctx := context.Background()
	err = d.sessionStore.StoreSubscription(ctx, d.memSession.ClientId(), sub)
	if err != nil {
		return err
	}
	_ = d.memSession.AddTopic(topic, qos)
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

func (d *dbSession) Topics() ([]string, []byte, error) {
	return d.memSession.Topics()
}

func (d *dbSession) ID() string {
	return d.memSession.ID()
}

func (d *dbSession) IDs() []byte {
	return d.memSession.IDs()
}

func (d *dbSession) Cmsg() *messagev5.ConnectMessage {
	return d.memSession.Cmsg()
}

func (d *dbSession) Will() *messagev5.PublishMessage {
	return d.memSession.Will()
}

func (d *dbSession) Pub1ack() sessionsv5.Ackqueue {
	return d.memSession.Pub1ack()
}

func (d *dbSession) Pub2in() sessionsv5.Ackqueue {
	return d.memSession.Pub2in()
}

func (d *dbSession) Pub2out() sessionsv5.Ackqueue {
	return d.memSession.Pub2out()
}

func (d *dbSession) Suback() sessionsv5.Ackqueue {
	return d.memSession.Suback()
}

func (d *dbSession) Unsuback() sessionsv5.Ackqueue {
	return d.memSession.Unsuback()
}

func (d *dbSession) Pingack() sessionsv5.Ackqueue {
	return d.memSession.Pingack()
}

func (d *dbSession) ExpiryInterval() uint32 {
	return d.memSession.ExpiryInterval()
}

func (d *dbSession) Status() sessionsv5.Status {
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

func (d *dbSession) SetStatus(status sessionsv5.Status) {
	d.memSession.SetStatus(status)
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
func (this *dbSession) SetWill(will *messagev5.PublishMessage) {
	this.memSession.SetWill(will)
}

func (this *dbSession) SetSub(sub *messagev5.SubscribeMessage) {
	this.memSession.SetSub(sub)
}
