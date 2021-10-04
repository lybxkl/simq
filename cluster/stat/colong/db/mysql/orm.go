package mysql

import (
	"encoding/hex"
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"gitee.com/Ljolan/si-mqtt/logger"
	"github.com/jinzhu/gorm"
	_ "github.com/jinzhu/gorm/dialects/mysql"
	"strings"
	"time"
)

type mysqlOrm struct {
	curName  string
	maxPubId int64
	maxSubId int64
	db       *gorm.DB
}

func newMysqlOrm(curName, url string, maxConn int) *mysqlOrm {
	db, err := gorm.Open("mysql", url)
	if err != nil {
		panic(err)
	}
	// 自动迁移
	db.AutoMigrate(&Message{})
	db.AutoMigrate(&Sub{})

	maxSubId := getMaxSubId(db)
	// TODO 需要初始化本地共享订阅数据

	maxPubId := getMaxPubId(db)
	db.DB().SetMaxOpenConns(maxConn)
	return &mysqlOrm{
		curName:  curName,
		maxPubId: maxPubId,
		maxSubId: maxSubId,
		db:       db,
	}
}

func getMaxSubId(db *gorm.DB) int64 {
	sub := &Sub{}
	d1 := db.Raw("select max(id) as idMax from sub").Select("idMax")
	r, err := d1.Rows()
	if err != nil {
		panic(err)
	}
	maxSubId := int64(0)
	for r.Next() {
		err = r.Scan(&sub.Id)
		if err != nil {
			logger.Logger.Warn(err)
		}
		maxSubId = sub.Id
	}
	return maxSubId
}

func getMaxPubId(db *gorm.DB) int64 {
	pub := &Message{}
	d2 := db.Raw("select max(id) as idMax from pub").Select("idMax")
	r1, err := d2.Rows()
	if err != nil {
		panic(err)
	}
	maxPubId := int64(0)
	for r1.Next() {
		err = r1.Scan(&pub.Id)
		if err != nil {
			logger.Logger.Warn(err)
		}
		maxPubId = pub.Id
	}
	return maxPubId
}
func (this *mysqlOrm) SaveSub(message *messagev5.SubscribeMessage) error {
	return this.db.Save(voToPoSub(this.curName, message)).Error
}
func (this *mysqlOrm) SaveUnSub(message *messagev5.UnsubscribeMessage) error {
	return this.db.Save(voToPoUnSub(this.curName, message)).Error
}
func (this *mysqlOrm) SaveSharePub(target, shareName string, message *messagev5.PublishMessage) error {
	return this.db.Save(voToPo(this.curName, target, shareName, message)).Error
}
func (this *mysqlOrm) SavePub(message *messagev5.PublishMessage) error {
	return this.db.Save(voToPo(this.curName, "", "", message)).Error
}
func (this *mysqlOrm) GetPubBatch(size int64) ([]Message, error) {
	data := make([]Message, 0)
	b := this.db.Raw("select * from pub where id > ? and sender != ? order by id asc limit ?", this.maxPubId, this.curName, size)
	if r, err := b.Rows(); err != nil {
		return nil, err
	} else {
		for r.Next() {
			m := Message{}
			e := b.ScanRows(r, &m)
			if e != nil {
				logger.Logger.Warn(err)
			} else {
				data = append(data, m)
			}
		}
	}
	if len(data) > 0 {
		this.maxPubId = data[len(data)-1].Id
	}
	return data, nil
}
func (this *mysqlOrm) GetSubBatch(size int64) ([]Sub, error) {
	data := make([]Sub, 0)
	b := this.db.Raw("select * from sub where id > ? and sender != ? order by id asc limit ?", this.maxSubId, this.curName, size)
	if r, err := b.Rows(); err != nil {
		return nil, err
	} else {
		for r.Next() {
			m := Sub{}
			e := b.ScanRows(r, &m)
			if e != nil {
				logger.Logger.Warn(err)
			} else {
				data = append(data, m)
			}
		}
	}
	if len(data) > 0 {
		this.maxSubId = data[len(data)-1].Id
	}
	return data, nil
}
func voToPoSub(sender string, message *messagev5.SubscribeMessage) *Sub {
	tps := message.Topics()
	return toSub(sender, 1, tps)
}
func toSub(sender string, subOrUnSub int, tps [][]byte) *Sub {
	tp := ""
	for i := 0; i < len(tps); i++ {
		tp += hex.EncodeToString(tps[i])
		tp += "."
	}
	if len(tp) > 0 {
		tp = tp[:len(tp)-1]
	}
	return &Sub{
		SubOrUnSub: subOrUnSub,
		Sender:     sender,
		Topic:      tp,
		Stamp:      time.Now().UnixNano(),
	}
}

type Sub struct {
	Id         int64  `gorm:"primary_key"`
	SubOrUnSub int    `gorm:"sub_or_unsub"` // 1: sub 2: unSub
	Sender     string `gorm:"sender"`
	Topic      string `gorm:"topic"` // 通过转为hex字符串拼接,号存储
	Stamp      int64  `gorm:"stamp,index"`
}

func (s Sub) TableName() string {
	return "sub"
}

// Message 代表 pub、retain、will
type Message struct {
	Id        int64  `gorm:"primary_key"`
	Sender    string `gorm:"index:sender_idx"`
	Target    string `gorm:"target"`
	ShareName string `gorm:"share_name"`
	Stamp     int64  `gorm:"stamp,index"`

	ClientId        string `gorm:"client_id"` // 客户端id
	Mtypeflags      uint8  `gorm:"mtypeflags"`
	Topic           string `gorm:"topic"`
	Qos             uint8  `gorm:"qos"`
	Payload         string `gorm:"payload"`
	PackageId       uint16 `gorm:"pk_id"`
	PfInd           uint8  `gorm:"pf_ind"`                              // 载荷格式指示，默认0 ， 服务端必须把接收到的应用消息中的载荷格式指示原封不动的发给所有的订阅者
	MsgExpiry       uint32 `gorm:"msg_expiry"`                          // 消息过期间隔，没有过期间隔，则应用消息不会过期 如果消息过期间隔已过期，服务端还没开始向匹配的订阅者交付该消息，则服务端必须删除该订阅者的消息副本
	TopicAlias      uint16 `gorm:"topic_alias"`                         // 主题别名，可以没有传，传了就不能为0，并且不能发超过connack中主题别名最大值
	ResponseTopic   string `gorm:"response_topic"`                      // 响应主题
	CorrelationData string `gorm:"correlation_data;type:varbinary(50)"` // 对比数据，字节类型，直接转为hex字符串存储
	UserProperty    string `gorm:"user_property"`                       // 用户属性 , 保证顺序，通过转为hex字符串拼接,号存储
	SubId           uint32 `gorm:"sub_id"`                              // 一个变长字节整数表示的订阅标识符 从1到268,435,455。订阅标识符的值为0将造成协议错误
	ContentType     string `gorm:"content_type"`                        // 内容类型 UTF-8编码
}

func (s Message) TableName() string {
	return "pub"
}
func voToPo(sender, target, shareName string, message *messagev5.PublishMessage) *Message {
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
	return &Message{
		Sender: sender,
		Stamp:  time.Now().UnixNano(),

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
func voToPoUnSub(sender string, message *messagev5.UnsubscribeMessage) *Sub {
	tps := message.Topics()
	return toSub(sender, 2, tps)
}
func poToBytes(message *Sub) [][]byte {
	tp := message.Topic
	tps := strings.Split(tp, ",")
	topics := make([][]byte, len(tps))
	for i := 0; i < len(tps); i++ {
		topics[i], _ = hex.DecodeString(tps[i])
	}
	return topics
}
