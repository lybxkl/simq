package mysqlrepo

import (
	"context"
	"encoding/hex"
	"fmt"
	store2 "gitee.com/Ljolan/si-mqtt/cluster/store"
	"gitee.com/Ljolan/si-mqtt/cluster/store/mysqlImpl/orm"
	"gitee.com/Ljolan/si-mqtt/cluster/store/mysqlImpl/orm/mysql"
	"gitee.com/Ljolan/si-mqtt/cluster/store/mysqlImpl/po"
	"gitee.com/Ljolan/si-mqtt/config"
	messagev52 "gitee.com/Ljolan/si-mqtt/corev5/v2/message"
	"gitee.com/Ljolan/si-mqtt/utils"
	"strings"
)

type MessageRepo struct {
	c orm.SiOrm
}

func NewMessageStore() store2.MessageStore {
	return &MessageRepo{}
}
func (m *MessageRepo) Start(ctx context.Context, config config.SIConfig) error {
	var err error
	storeCfg := config.Store.Mysql
	m.c, err = mysql.NewMysqlOrm(storeCfg.Source, storeCfg.PoolSize)
	utils.MustPanic(m.c.AutoMigrate(&po.Will{}))
	utils.MustPanic(m.c.AutoMigrate(&po.Retain{}))
	utils.MustPanic(m.c.AutoMigrate(&po.Inflow{}))
	utils.MustPanic(m.c.AutoMigrate(&po.Outflow{}))
	utils.MustPanic(m.c.AutoMigrate(&po.MessagePk{}))
	utils.MustPanic(m.c.AutoMigrate(&po.Offline{}))
	utils.MustPanic(m.c.AutoMigrate(&po.Session{}))
	utils.MustPanic(m.c.AutoMigrate(&po.Subscription{}))

	return err
}

func (m *MessageRepo) Stop(ctx context.Context) error {
	return nil
}

func (m *MessageRepo) StoreWillMessage(ctx context.Context, clientId string, message *messagev52.PublishMessage) error {
	return m.c.Save(ctx, "si_will", "client_id", "", voToPoWill(clientId, message))
}

func (m *MessageRepo) ClearWillMessage(ctx context.Context, clientId string) error {
	return m.c.Delete(ctx, "si_will", wrapperOne("client_id", clientId), &po.Will{})
}

func (m *MessageRepo) GetWillMessage(ctx context.Context, clientId string) (*messagev52.PublishMessage, error) {
	ret := make([]po.Will, 0)
	err := m.c.Get(ctx, "si_will", wrapperOne("client_id", clientId), &ret)
	if err != nil || len(ret) == 0 {
		return nil, err
	}
	return poToVoWill(ret[0]), nil
}

func (m *MessageRepo) StoreRetainMessage(ctx context.Context, topic string, message *messagev52.PublishMessage) error {
	rt := voToPoRetain(topic, message)
	return m.c.Save(ctx, "si_retain", "client_id", "", rt)
}

func (m *MessageRepo) ClearRetainMessage(ctx context.Context, topic string) error {
	return m.c.Delete(ctx, "si_retain", wrapperOne("topic", topic), &po.Retain{})
}

func (m *MessageRepo) GetRetainMessage(ctx context.Context, topic string) (*messagev52.PublishMessage, error) {
	ret := make([]po.Retain, 0)
	err := m.c.Get(ctx, "si_retain", wrapperOne("topic", topic), &ret)
	if err != nil || len(ret) == 0 {
		return nil, err
	}
	return poToVoRetain(ret[0]), nil
}

func (m *MessageRepo) GetAllRetainMsg(ctx context.Context) ([]*messagev52.PublishMessage, error) {
	data := make([]po.Retain, 0)
	err := m.c.Get(ctx, "si_retain", "", &data)
	if err != nil || len(data) == 0 {
		return nil, err
	}
	ret := make([]*messagev52.PublishMessage, len(data))
	for i := 0; i < len(ret); i++ {
		ret[i] = poToVoRetain(data[i])
	}
	return ret, nil
}
func voToPo(id string, message *messagev52.PublishMessage) po.Message {
	up := message.UserProperty()
	var ups string
	if up != nil && len(up) >= 0 {
		for i := 0; i < len(up); i++ {
			ups += hex.EncodeToString(up[i])
			ups += ","
		}
	}
	if len(ups) > 0 {
		ups = ups[:len(ups)-1]
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
		CorrelationData: hex.EncodeToString(message.CorrelationData()),
		UserProperty:    ups,
		SubId:           message.SubscriptionIdentifier(),
		ContentType:     string(message.ContentType()),
	}
}
func voToPoWill(id string, message *messagev52.PublishMessage) po.Will {
	m := voToPo(id, message)
	return po.Will{
		ClientId:        m.ClientId,
		Mtypeflags:      m.Mtypeflags,
		Topic:           m.Topic,
		Qos:             m.Qos,
		Payload:         m.Payload,
		PfInd:           m.PfInd,
		MsgExpiry:       m.MsgExpiry,
		TopicAlias:      m.TopicAlias,
		ResponseTopic:   m.ResponseTopic,
		CorrelationData: m.CorrelationData,
		UserProperty:    m.UserProperty,
		SubId:           m.SubId,
		ContentType:     m.ContentType,
	}
}
func voToPoRetain(id string, message *messagev52.PublishMessage) po.Retain {
	m := voToPo(id, message)
	return po.Retain{
		Mtypeflags:      m.Mtypeflags,
		Topic:           m.Topic,
		Qos:             m.Qos,
		Payload:         m.Payload,
		PfInd:           m.PfInd,
		MsgExpiry:       m.MsgExpiry,
		TopicAlias:      m.TopicAlias,
		ResponseTopic:   m.ResponseTopic,
		CorrelationData: m.CorrelationData,
		UserProperty:    m.UserProperty,
		SubId:           m.SubId,
		ContentType:     m.ContentType,
	}
}
func voToPoInflow(id string, message *messagev52.PublishMessage) po.Inflow {
	return po.Inflow(voToPo(id, message))
}
func voToPoOutflow(id string, message *messagev52.PublishMessage) po.Outflow {
	return po.Outflow(voToPo(id, message))
}
func voToPoOffline(id string, message *messagev52.PublishMessage) po.Offline {
	return po.Offline(voToPo(id, message))
}
func poToVoWill(message po.Will) *messagev52.PublishMessage {
	return poToVo(po.Message{
		ClientId:        message.ClientId,
		Mtypeflags:      message.Mtypeflags,
		Topic:           message.Topic,
		Qos:             message.Qos,
		Payload:         message.Payload,
		PfInd:           message.PfInd,
		MsgExpiry:       message.MsgExpiry,
		TopicAlias:      message.TopicAlias,
		ResponseTopic:   message.ResponseTopic,
		CorrelationData: message.CorrelationData,
		UserProperty:    message.UserProperty,
		SubId:           message.SubId,
		ContentType:     message.ContentType,
	})
}
func poToVoRetain(message po.Retain) *messagev52.PublishMessage {
	return poToVo(po.Message{
		Mtypeflags:      message.Mtypeflags,
		Topic:           message.Topic,
		Qos:             message.Qos,
		Payload:         message.Payload,
		PfInd:           message.PfInd,
		MsgExpiry:       message.MsgExpiry,
		TopicAlias:      message.TopicAlias,
		ResponseTopic:   message.ResponseTopic,
		CorrelationData: message.CorrelationData,
		UserProperty:    message.UserProperty,
		SubId:           message.SubId,
		ContentType:     message.ContentType,
	})
}
func poToVoInflow(message po.Inflow) *messagev52.PublishMessage {
	return poToVo(po.Message(message))
}
func poToVoOutflow(message po.Outflow) *messagev52.PublishMessage {
	return poToVo(po.Message(message))
}
func poToVoOffline(message po.Offline) *messagev52.PublishMessage {
	return poToVo(po.Message(message))
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
	if message.CorrelationData != "" {
		b, er := hex.DecodeString(message.CorrelationData)
		if er != nil {
			// todo
		}
		pub.SetCorrelationData(b)
	}
	up := message.UserProperty
	if len(up) > 0 {
		upsp := strings.Split(up, ",")
		ups := make([][]byte, len(upsp))
		for i := 0; i < len(upsp); i++ {
			ups[i], _ = hex.DecodeString(upsp[i])
		}
		pub.AddUserPropertys(ups)
	}
	pub.SetSubscriptionIdentifier(message.SubId)
	pub.SetContentType([]byte(message.ContentType))
	return pub
}

func inMsgs(clientId string, msgs []string) string {
	var ins strings.Builder
	ins.WriteString(fmt.Sprintf("client_id = '%s' ", clientId))
	if len(msgs) > 0 {
		ins.WriteString(" and msg_id in (")
	}
	pks := make([]interface{}, len(msgs))
	for i := 0; i < len(msgs); i++ {
		if i == len(msgs)-1 {
			ins.WriteString("'%v')")
		} else {
			ins.WriteString("'%v',")
		}
		pks[i] = msgs[i]
	}
	return fmt.Sprintf(ins.String(), pks...)
}

func inPks(clientId string, pkId []uint16) string {
	var ins strings.Builder
	ins.WriteString(fmt.Sprintf("client_id = '%s' ", clientId))
	if len(pkId) > 0 {
		ins.WriteString(" and pk_id in (")
	}
	pks := make([]interface{}, len(pkId))
	for i := 0; i < len(pkId); i++ {
		if i == len(pkId)-1 {
			ins.WriteString("%v)")
		} else {
			ins.WriteString("%v,")
		}
		pks[i] = pkId[i]
	}
	return fmt.Sprintf(ins.String(), pks...)
}

func wrapperOne(k, v string) string {
	return fmt.Sprintf("%s = '%s'", k, v)
}
