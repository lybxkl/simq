package po

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

type MessagePk struct {
	ClientId string `bson:"client_id"`
	PkId     uint16 `bson:"pk_id"`
	OpTime   int64  `bson:"op_time"`
}
