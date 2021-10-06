package mysql

import (
	"encoding/hex"
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"gitee.com/Ljolan/si-mqtt/logger"
	"gitee.com/Ljolan/si-mqtt/utils"
	"gorm.io/driver/mysql"
	"gorm.io/gorm"
	glog "gorm.io/gorm/logger"
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
	cfg := &gorm.Config{
		SkipDefaultTransaction: true,
		Logger:                 glog.Default.LogMode(glog.Error),
		PrepareStmt:            true,
		DisableAutomaticPing:   true,
	}
	db, err := gorm.Open(mysql.Open(url), cfg)
	utils.MustPanic(err)
	// 自动迁移
	utils.MustPanic(db.AutoMigrate(&Message{}))
	utils.MustPanic(db.AutoMigrate(&Sub{}))

	// 启动时，每次都拉取全部，因为会合并记录
	// 就完成启动时同步集群共享订阅数据了
	maxSubId := int64(0) // getMaxSubId(db)

	maxPubId := getMaxPubId(db)
	sqlDB, e := db.DB()
	utils.MustPanic(e)

	sqlDB.SetMaxOpenConns(maxConn)
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
	utils.MustPanic(err)
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
	utils.MustPanic(err)
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
				logger.Logger.Warn(e)
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
				logger.Logger.Warn(e)
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
	SubOrUnSub int    `gorm:"column:sub_or_unsub"` // 1: sub 2: unSub
	Sender     string `gorm:"column:sender;type:varchar(30)"`
	Topic      string `gorm:"column:topic;type:varchar(80)"` // 通过转为hex字符串拼接,号存储
	Num        uint32 `gorm:"column:num"`                    // 合并的数据，默认0表示一个，其它表示 值+1 如， 2 表示 2+1=3
	Stamp      int64  `gorm:"column:stamp;index"`
}

func (s Sub) TableName() string {
	return "sub"
}

// Message 代表 pub、retain、will
type Message struct {
	Id        int64  `gorm:"primary_key"`
	Sender    string `gorm:"column:index:sender_idx"`
	Target    string `gorm:"column:target"`
	ShareName string `gorm:"column:share_name"`
	Stamp     int64  `gorm:"column:stamp;index"`

	ClientId        string `gorm:"column:client_id"` // 客户端id
	Mtypeflags      uint8  `gorm:"column:mtypeflags"`
	Topic           string `gorm:"column:topic"`
	Qos             uint8  `gorm:"column:qos"`
	Payload         string `gorm:"column:payload"`
	PackageId       uint16 `gorm:"column:pk_id"`
	PfInd           uint8  `gorm:"column:pf_ind"`                              // 载荷格式指示，默认0 ， 服务端必须把接收到的应用消息中的载荷格式指示原封不动的发给所有的订阅者
	MsgExpiry       uint32 `gorm:"column:msg_expiry"`                          // 消息过期间隔，没有过期间隔，则应用消息不会过期 如果消息过期间隔已过期，服务端还没开始向匹配的订阅者交付该消息，则服务端必须删除该订阅者的消息副本
	TopicAlias      uint16 `gorm:"column:topic_alias"`                         // 主题别名，可以没有传，传了就不能为0，并且不能发超过connack中主题别名最大值
	ResponseTopic   string `gorm:"column:response_topic"`                      // 响应主题
	CorrelationData string `gorm:"column:correlation_data;type:varbinary(50)"` // 对比数据，字节类型，直接转为hex字符串存储
	UserProperty    string `gorm:"column:user_property"`                       // 用户属性 , 保证顺序，通过转为hex字符串拼接,号存储
	SubId           uint32 `gorm:"column:sub_id"`                              // 一个变长字节整数表示的订阅标识符 从1到268,435,455。订阅标识符的值为0将造成协议错误
	ContentType     string `gorm:"column:content_type"`                        // 内容类型 UTF-8编码
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
