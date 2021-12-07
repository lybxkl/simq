package mongorepo

import (
	"context"
	store2 "gitee.com/Ljolan/si-mqtt/cluster/store"
	"gitee.com/Ljolan/si-mqtt/cluster/store/mongoImpl/orm"
	"gitee.com/Ljolan/si-mqtt/cluster/store/mongoImpl/orm/mongo"
	"gitee.com/Ljolan/si-mqtt/cluster/store/mongoImpl/po"
	"gitee.com/Ljolan/si-mqtt/config"
	messagev52 "gitee.com/Ljolan/si-mqtt/corev5/v2/message"
)

type MessageRepo struct {
	c orm.SiOrm
}

func NewMessageStore() store2.MessageStore {
	return &MessageRepo{}
}
func (m *MessageRepo) Start(ctx context.Context, config config.SIConfig) error {
	var err error
	storeCfg := config.Store.Mongo
	m.c, err = mongoorm.NewMongoOrm(storeCfg.Source, storeCfg.MinPoolSize, storeCfg.MaxPoolSize, storeCfg.MaxConnIdleTime)
	return err
}

func (m *MessageRepo) Stop(ctx context.Context) error {
	return nil
}

func (m *MessageRepo) StoreWillMessage(ctx context.Context, clientId string, message *messagev52.PublishMessage) error {
	return m.c.Save(ctx, "si_will", clientId, voToPo(clientId, message))
}

func (m *MessageRepo) ClearWillMessage(ctx context.Context, clientId string) error {
	return m.c.Delete(ctx, "si_will", orm.Select{"_id": clientId})
}

func (m *MessageRepo) GetWillMessage(ctx context.Context, clientId string) (*messagev52.PublishMessage, error) {
	ret := make([]po.Message, 0)
	err := m.c.Get(ctx, "si_will", orm.Select{"_id": clientId}, &ret)
	if err != nil || len(ret) == 0 {
		return nil, err
	}
	return poToVo(ret[0]), nil
}

func (m *MessageRepo) StoreRetainMessage(ctx context.Context, topic string, message *messagev52.PublishMessage) error {
	rt := voToPo(topic, message)
	rt.ClientId = ""
	return m.c.Save(ctx, "si_retain", topic, rt)
}

func (m *MessageRepo) ClearRetainMessage(ctx context.Context, topic string) error {
	return m.c.Delete(ctx, "si_retain", orm.Select{"_id": topic})
}

func (m *MessageRepo) GetRetainMessage(ctx context.Context, topic string) (*messagev52.PublishMessage, error) {
	ret := make([]po.Message, 0)
	err := m.c.Get(ctx, "si_retain", orm.Select{"_id": topic}, &ret)
	if err != nil || len(ret) == 0 {
		return nil, err
	}
	return poToVo(ret[0]), nil
}

func (m *MessageRepo) GetAllRetainMsg(ctx context.Context) ([]*messagev52.PublishMessage, error) {
	data := make([]po.Message, 0)
	err := m.c.Get(ctx, "si_retain", orm.Select{}, &data)
	if err != nil || len(data) == 0 {
		return nil, err
	}
	ret := make([]*messagev52.PublishMessage, len(data))
	for i := 0; i < len(ret); i++ {
		ret[i] = poToVo(data[i])
	}
	return ret, nil
}

func voToPo(id string, message *messagev52.PublishMessage) po.Message {
	up := message.UserProperty()
	var ups []string
	if up != nil && len(up) >= 0 {
		ups = make([]string, len(up))
		for i := 0; i < len(up); i++ {
			ups[i] = string(up[i])
		}
	}
	return po.Message{
		ClientId:        id,
		Mtypeflags:      message.MtypeFlags(),
		Topic:           string(message.Topic()),
		Qos:             message.QoS(),
		Payload:         string(message.Payload()),
		PackageId:       message.PacketId(),
		PfInd:           message.PayloadFormatIndicator(),
		MsgExpiry:       message.MessageExpiryInterval(),
		TopicAlias:      message.TopicAlias(),
		ResponseTopic:   string(message.ResponseTopic()),
		CorrelationData: message.CorrelationData(),
		UserProperty:    ups,
		SubId:           message.SubscriptionIdentifier(),
		ContentType:     string(message.ContentType()),
	}
}
func poToVo(message po.Message) *messagev52.PublishMessage {
	pub := messagev52.NewPublishMessage()
	pub.SetMtypeFlags(message.Mtypeflags)
	_ = pub.SetTopic([]byte(message.Topic))
	_ = pub.SetQoS(message.Qos)
	pub.SetPayload([]byte(message.Payload))
	pub.SetPacketId(message.PackageId)
	pub.SetPayloadFormatIndicator(message.PfInd)
	pub.SetMessageExpiryInterval(message.MsgExpiry)
	pub.SetTopicAlias(message.TopicAlias)
	pub.SetResponseTopic([]byte(message.ResponseTopic))
	pub.SetCorrelationData(message.CorrelationData)
	if len(message.UserProperty) > 0 {
		ups := make([][]byte, len(message.UserProperty))
		for i := 0; i < len(message.UserProperty); i++ {
			ups[i] = []byte(message.UserProperty[i])
		}
		pub.AddUserPropertys(ups)
	}
	pub.SetSubscriptionIdentifier(message.SubId)
	pub.SetContentType([]byte(message.ContentType))
	return pub
}
