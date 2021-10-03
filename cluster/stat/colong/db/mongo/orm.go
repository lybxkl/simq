package mongo

import (
	"context"
	"fmt"
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

type mongoOrm struct {
	db       *mongo.Database
	nodeName string             // 当前节点名称
	maxObjId primitive.ObjectID // 时间戳
}

func newMongoOrm(nodeName, url string, minPool, maxPool, maxConnIdle uint64) (*mongoOrm, error) {
	// 设置mongoDB客户端连接信息
	param := fmt.Sprintf(url)
	clientOptions := options.Client().ApplyURI(param).SetMinPoolSize(minPool).SetMaxPoolSize(maxPool).SetMaxConnIdleTime(time.Duration(maxConnIdle) * time.Second)

	// 建立客户端连接
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		return nil, err
	}

	// 检查连接情况
	err = client.Ping(context.TODO(), nil)
	if err != nil {
		return nil, err
	}
	fmt.Println("Connected to MongoDB!")

	cli := client.Database("simq")

	v := &MessagePo{}
	op := &options.FindOneOptions{}
	op.SetSort(bson.M{"_id": -1})
	c := cli.Collection("cluster_msg").FindOne(context.TODO(), bson.M{}, op)
	err = c.Decode(v)
	if err != nil && err.Error() != "EOF" {
		panic(err)
	}
	println(v.Id.String())
	return &mongoOrm{
		db:       cli,
		nodeName: nodeName,
		maxObjId: v.Id,
	}, nil
}

type MessagePo struct {
	Id         primitive.ObjectID `bson:"_id,omitempty"`
	Sender     string             `bson:"sender,omitempty"`
	Target     string             `bson:"target,omitempty"`
	ShareName  string             `bson:"share_name,omitempty"`
	SubOrUnSub int                `bson:"sub_or_unsub,omitempty"` // 1: sub 2: unSub
	Sub        *Sub               `bson:"sub,omitempty"`
	Msg        *Message           `bson:"msg,omitempty"`
	Stamp      int64              `bson:"stamp,index"`
}
type Sub struct {
	Topic []string `bson:"topic"`
}

func (p *MessagePo) IsPub() bool {
	if p.Target == "" && p.SubOrUnSub == 0 && p.Msg != nil {
		return true
	}
	return false
}
func (p *MessagePo) IsShare() bool {
	if p.Target != "" && p.ShareName != "" && p.Msg != nil {
		return true
	}
	return false
}
func (p *MessagePo) IsSub() bool {
	if p.SubOrUnSub == 1 && p.Sub != nil {
		return true
	}
	return false
}
func (p *MessagePo) IsUnSub() bool {
	if p.SubOrUnSub == 2 && p.Sub != nil {
		return true
	}
	return false
}

func (m *mongoOrm) SaveSub(ctx context.Context, tab string, msg *messagev5.SubscribeMessage) error {
	var (
		sub        = &Sub{}
		subOrUnSub = 1
		tps        = msg.Topics()
	)
	for i := 0; i < len(tps); i++ {
		sub.Topic = append(sub.Topic, string(tps[i]))
	}
	_, err := m.db.Collection(tab).InsertOne(ctx, &MessagePo{
		Sender:     m.nodeName,
		SubOrUnSub: subOrUnSub,
		Sub:        sub,
		Stamp:      time.Now().UnixNano(),
	})
	return err
}
func (m *mongoOrm) SaveUnSub(ctx context.Context, tab string, msg *messagev5.UnsubscribeMessage) error {
	var (
		sub        = &Sub{}
		subOrUnSub = 2
		tps        = msg.Topics()
	)
	for i := 0; i < len(tps); i++ {
		sub.Topic = append(sub.Topic, string(tps[i]))
	}
	_, err := m.db.Collection(tab).InsertOne(ctx, &MessagePo{
		Sender:     m.nodeName,
		SubOrUnSub: subOrUnSub,
		Sub:        sub,
		Stamp:      time.Now().UnixNano(),
	})
	return err
}
func (m *mongoOrm) SavePub(ctx context.Context, tab string, msg *messagev5.PublishMessage) error {
	_, err := m.db.Collection(tab).InsertOne(ctx, &MessagePo{
		Sender: m.nodeName,
		Msg:    voToPo("", msg),
		Stamp:  time.Now().UnixNano(),
	})
	return err
}
func (m *mongoOrm) SaveSharePub(ctx context.Context, tab string, target, shareName string, msg *messagev5.PublishMessage) error {
	_, err := m.db.Collection(tab).InsertOne(ctx, &MessagePo{
		Sender:    m.nodeName,
		Target:    target,
		ShareName: shareName,
		Msg:       voToPo("", msg),
		Stamp:     time.Now().UnixNano(),
	})
	return err
}
func (m *mongoOrm) GetBatch(ctx context.Context, tab string, size int64) ([]MessagePo, error) {
	op := &options.FindOptions{}
	op.SetLimit(size)
	op.SetSort(bson.M{"_id": 1})
	filter := bson.M{}
	filter["_id"] = bson.M{"$gt": m.maxObjId}
	filter["sender"] = bson.M{"$ne": m.nodeName}
	c, e := m.db.Collection(tab).Find(ctx, filter, op)
	if e != nil {
		return nil, e
	}
	r := make([]MessagePo, 0)
	e = c.All(ctx, &r)
	if e != nil {
		return nil, e
	}
	if len(r) > 0 {
		m.maxObjId = r[len(r)-1].Id
	}
	return r, e
}

// Message 代表 pub、retain、will
type Message struct {
	ClientId        string   `bson:"client_id"` // 客户端id
	MsgId           string   `bson:"msg_id"`
	Mtypeflags      uint8    `bson:"mtypeflags"`
	Topic           string   `bson:"topic"`
	Qos             uint8    `bson:"qos"`
	Payload         string   `bson:"payload"`
	PackageId       uint16   `bson:"pk_id"`
	PfInd           uint8    `bson:"pf_ind"`           // 载荷格式指示，默认0 ， 服务端必须把接收到的应用消息中的载荷格式指示原封不动的发给所有的订阅者
	MsgExpiry       uint32   `bson:"msg_expiry"`       // 消息过期间隔，没有过期间隔，则应用消息不会过期 如果消息过期间隔已过期，服务端还没开始向匹配的订阅者交付该消息，则服务端必须删除该订阅者的消息副本
	TopicAlias      uint16   `bson:"topic_alias"`      // 主题别名，可以没有传，传了就不能为0，并且不能发超过connack中主题别名最大值
	ResponseTopic   string   `bson:"response_topic"`   // 响应主题
	CorrelationData []uint8  `bson:"correlation_data"` // 对比数据，字节类型
	UserProperty    []string `bson:"user_property"`    // 用户属性 , 保证顺序
	SubId           uint32   `bson:"sub_id"`           // 一个变长字节整数表示的订阅标识符 从1到268,435,455。订阅标识符的值为0将造成协议错误
	ContentType     string   `bson:"content_type"`     // 内容类型 UTF-8编码
}

func voToPo(id string, message *messagev5.PublishMessage) *Message {
	up := message.UserProperty()
	var ups []string
	if up != nil && len(up) >= 0 {
		ups = make([]string, len(up))
		for i := 0; i < len(up); i++ {
			ups[i] = string(up[i])
		}
	}
	return &Message{
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
func poToVo(message *Message) *messagev5.PublishMessage {
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
