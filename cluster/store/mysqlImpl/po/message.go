package po

// Message 代表 pub、retain、will
type Message struct {
	ClientId        string `gorm:"index:cid"` // 客户端id
	MsgId           string `gorm:"column:msg_id"`
	Mtypeflags      uint8  `gorm:"column:mtypeflags"`
	Topic           string `gorm:"column:topic"`
	Qos             uint8  `gorm:"column:qos"`
	Payload         string `gorm:"column:payload"`
	PackageId       uint16 `gorm:"column:pk_id;index:pk_id"`
	PfInd           uint8  `gorm:"column:pf_ind"`           // 载荷格式指示，默认0 ， 服务端必须把接收到的应用消息中的载荷格式指示原封不动的发给所有的订阅者
	MsgExpiry       uint32 `gorm:"column:msg_expiry"`       // 消息过期间隔，没有过期间隔，则应用消息不会过期 如果消息过期间隔已过期，服务端还没开始向匹配的订阅者交付该消息，则服务端必须删除该订阅者的消息副本
	TopicAlias      uint16 `gorm:"column:topic_alias"`      // 主题别名，可以没有传，传了就不能为0，并且不能发超过connack中主题别名最大值
	ResponseTopic   string `gorm:"column:response_topic"`   // 响应主题
	CorrelationData string `gorm:"column:correlation_data"` // 对比数据，字节类型 []uint8 --> hex string
	UserProperty    string `gorm:"column:user_property"`    // 用户属性 , 保证顺序 []string --> hex string join ,
	SubId           uint32 `gorm:"column:sub_id"`           // 一个变长字节整数表示的订阅标识符 从1到268,435,455。订阅标识符的值为0将造成协议错误
	ContentType     string `gorm:"column:content_type"`     // 内容类型 UTF-8编码
}
type Will struct {
	ClientId        string `gorm:"primary_key"` // 客户端id
	Mtypeflags      uint8  `gorm:"column:mtypeflags"`
	Topic           string `gorm:"column:topic"`
	Qos             uint8  `gorm:"column:qos"`
	Payload         string `gorm:"column:payload"`
	PfInd           uint8  `gorm:"column:pf_ind"`           // 载荷格式指示，默认0 ， 服务端必须把接收到的应用消息中的载荷格式指示原封不动的发给所有的订阅者
	MsgExpiry       uint32 `gorm:"column:msg_expiry"`       // 消息过期间隔，没有过期间隔，则应用消息不会过期 如果消息过期间隔已过期，服务端还没开始向匹配的订阅者交付该消息，则服务端必须删除该订阅者的消息副本
	TopicAlias      uint16 `gorm:"column:topic_alias"`      // 主题别名，可以没有传，传了就不能为0，并且不能发超过connack中主题别名最大值
	ResponseTopic   string `gorm:"column:response_topic"`   // 响应主题
	CorrelationData string `gorm:"column:correlation_data"` // 对比数据，字节类型 []uint8 --> hex string
	UserProperty    string `gorm:"column:user_property"`    // 用户属性 , 保证顺序 []string --> hex string join ,
	SubId           uint32 `gorm:"column:sub_id"`           // 一个变长字节整数表示的订阅标识符 从1到268,435,455。订阅标识符的值为0将造成协议错误
	ContentType     string `gorm:"column:content_type"`     // 内容类型 UTF-8编码
}

func (w Will) TableName() string {
	return "si_will"
}

type Retain struct {
	Mtypeflags      uint8  `gorm:"column:mtypeflags"`
	Topic           string `gorm:"column:primary_key"`
	Qos             uint8  `gorm:"column:qos"`
	Payload         string `gorm:"column:payload"`
	PfInd           uint8  `gorm:"column:pf_ind"`           // 载荷格式指示，默认0 ， 服务端必须把接收到的应用消息中的载荷格式指示原封不动的发给所有的订阅者
	MsgExpiry       uint32 `gorm:"column:msg_expiry"`       // 消息过期间隔，没有过期间隔，则应用消息不会过期 如果消息过期间隔已过期，服务端还没开始向匹配的订阅者交付该消息，则服务端必须删除该订阅者的消息副本
	TopicAlias      uint16 `gorm:"column:topic_alias"`      // 主题别名，可以没有传，传了就不能为0，并且不能发超过connack中主题别名最大值
	ResponseTopic   string `gorm:"column:response_topic"`   // 响应主题
	CorrelationData string `gorm:"column:correlation_data"` // 对比数据，字节类型 []uint8 --> hex string
	UserProperty    string `gorm:"column:user_property"`    // 用户属性 , 保证顺序 []string --> hex string join ,
	SubId           uint32 `gorm:"column:sub_id"`           // 一个变长字节整数表示的订阅标识符 从1到268,435,455。订阅标识符的值为0将造成协议错误
	ContentType     string `gorm:"column:content_type"`     // 内容类型 UTF-8编码
}

func (w Retain) TableName() string {
	return "si_retain"
}

type Inflow Message

func (w Inflow) TableName() string {
	return "si_inflow"
}

type Outflow Message

func (w Outflow) TableName() string {
	return "si_outflow"
}

type Offline Message

func (w Offline) TableName() string {
	return "si_offline"
}

type MessagePk struct {
	ClientId string `gorm:"column:client_id;index:cid"`
	PkId     uint16 `gorm:"column:pk_id;index:mid"`
	OpTime   int64  `gorm:"column:op_time"`
}

func (w MessagePk) TableName() string {
	return "si_outflowsec"
}
