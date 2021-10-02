package repo

import (
	"context"
	store2 "gitee.com/Ljolan/si-mqtt/cluster/store"
	orm2 "gitee.com/Ljolan/si-mqtt/cluster/store/orm"
	mongoorm2 "gitee.com/Ljolan/si-mqtt/cluster/store/orm/mongo"
	po2 "gitee.com/Ljolan/si-mqtt/cluster/store/po"
	"gitee.com/Ljolan/si-mqtt/config"
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
)

type MessageRepo struct {
	c orm2.SiOrm
}

func NewMessageStore() store2.MessageStore {
	return &MessageRepo{}
}
func (m *MessageRepo) Start(ctx context.Context, config config.SIConfig) error {
	var err error
	storeCfg := config.Store.Mongo
	m.c, err = mongoorm2.NewMongoOrm(storeCfg.Source, storeCfg.MinPoolSize, storeCfg.MaxPoolSize, storeCfg.MaxConnIdleTime)
	return err
}

func (m *MessageRepo) Stop(ctx context.Context) error {
	return nil
}

func (m *MessageRepo) StoreWillMessage(ctx context.Context, clientId string, message *messagev5.PublishMessage) error {
	return m.c.Save(ctx, "si_will", clientId, voToPo(clientId, message))
}

func (m *MessageRepo) ClearWillMessage(ctx context.Context, clientId string) error {
	return m.c.Delete(ctx, "si_will", orm2.Select{"_id": clientId})
}

func (m *MessageRepo) GetWillMessage(ctx context.Context, clientId string) (*messagev5.PublishMessage, error) {
	ret := make([]po2.Message, 0)
	err := m.c.Get(ctx, "si_will", orm2.Select{"_id": clientId}, &ret)
	if err != nil || len(ret) == 0 {
		return nil, err
	}
	return poToVo(ret[0]), nil
}

func (m *MessageRepo) StoreRetainMessage(ctx context.Context, topic string, message *messagev5.PublishMessage) error {
	rt := voToPo(topic, message)
	rt.ClientId = ""
	return m.c.Save(ctx, "si_retain", topic, rt)
}

func (m *MessageRepo) ClearRetainMessage(ctx context.Context, topic string) error {
	return m.c.Delete(ctx, "si_retain", orm2.Select{"_id": topic})
}

func (m *MessageRepo) GetRetainMessage(ctx context.Context, topic string) (*messagev5.PublishMessage, error) {
	ret := make([]po2.Message, 0)
	err := m.c.Get(ctx, "si_retain", orm2.Select{"_id": topic}, &ret)
	if err != nil || len(ret) == 0 {
		return nil, err
	}
	return poToVo(ret[0]), nil
}

func (m *MessageRepo) GetAllRetainMsg(ctx context.Context) ([]*messagev5.PublishMessage, error) {
	data := make([]po2.Message, 0)
	err := m.c.Get(ctx, "si_retain", orm2.Select{}, &data)
	if err != nil || len(data) == 0 {
		return nil, err
	}
	ret := make([]*messagev5.PublishMessage, len(data))
	for i := 0; i < len(ret); i++ {
		ret[i] = poToVo(data[i])
	}
	return ret, nil
}

func voToPo(id string, message *messagev5.PublishMessage) po2.Message {
	up := message.UserProperty()
	var ups []string
	if up != nil && len(up) >= 0 {
		ups = make([]string, len(up))
		for i := 0; i < len(up); i++ {
			ups[i] = string(up[i])
		}
	}
	return po2.Message{
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
func poToVo(message po2.Message) *messagev5.PublishMessage {
	pub := messagev5.NewPublishMessage()
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
