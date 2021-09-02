package po

type Pack struct {
	Id uint64 `gorm:"column:id" json:"id"`
	ClientId string `gorm:"column:client_id" json:"client_id"`
	PkId int `gorm:"column:pk_id" json:"pk_id"`
	PTime int64 `gorm:"column:ptime" json:"ptime"`
}

func (p Pack) TableName() string {
	return "pack"
}

type PubMsg struct {
	Id int64 `gorm:"column:id" json:"id"`
	ClientId string `gorm:"column:client_id" json:"client_id"`
	PkId uint `gorm:"column:pk_id" json:"pk_id"`
	MTypeFlags int `gorm:"column:mtypeflags" json:"mtypeflags"`
	Topic string `gorm:"column:topic" json:"topic"`
	PayloadFormatIndicator int8 `gorm:"column:payload_format_indicator" json:"payload_format_indicator"`
	MsgExpiryInterval uint64 `gorm:"column:msg_expiry_interval" json:"msg_expiry_interval"`
	TopicAlias uint `gorm:"column:topic_alias" json:"topic_alias"`
	ResponseTopic string `gorm:"column:response_topic" json:"response_topic"`
	CorrelationData string `gorm:"column:correlation_data" json:"correlation_data"`
	UserProperty string `gorm:"column:user_property" json:"user_property"`
	SubIdentifier int64 `gorm:"column:sub_identifier" json:"sub_identifier"`
	ContentType string `gorm:"column:content_type" json:"content_type"`
	Payload string `gorm:"column:payload" json:"payload"`
	PTime int64 `gorm:"column:ptime" json:"ptime"`
}
func (p PubMsg) TableName() string {
	return "pubmsg"
}
type RetainMsg struct {
	Id int64 `gorm:"column:id" json:"id"`
	ClientId string `gorm:"column:client_id" json:"client_id"`
	PkId uint `gorm:"column:pk_id" json:"pk_id"`
	MTypeFlags int `gorm:"column:mtypeflags" json:"mtypeflags"`
	Topic string `gorm:"column:topic" json:"topic"`
	PayloadFormatIndicator int8 `gorm:"column:payload_format_indicator" json:"payload_format_indicator"`
	MsgExpiryInterval uint64 `gorm:"column:msg_expiry_interval" json:"msg_expiry_interval"`
	TopicAlias uint `gorm:"column:topic_alias" json:"topic_alias"`
	ResponseTopic string `gorm:"column:response_topic" json:"response_topic"`
	CorrelationData string `gorm:"column:correlation_data" json:"correlation_data"`
	UserProperty string `gorm:"column:user_property" json:"user_property"`
	SubIdentifier int64 `gorm:"column:sub_identifier" json:"sub_identifier"`
	ContentType string `gorm:"column:content_type" json:"content_type"`
	Payload string `gorm:"column:payload" json:"payload"`
	RTime int64 `gorm:"column:rtime" json:"rtime"`
}
func (p RetainMsg) TableName() string {
	return "retainmsg"
}
type Sub struct {
	Id uint64 `gorm:"column:id" json:"id"`
	ClientId string `gorm:"column:client_id" json:"client_id"`
	Topic string `gorm:"column:topic" json:"topic"`
	Qos uint8 `gorm:"column:qos" json:"qos"`
	SubIdentifier uint `gorm:"column:sub_identifier" json:"sub_identifier"`
	STime int64 `gorm:"column:stime" json:"stime"`
}
func (p Sub) TableName() string {
	return "sub"
}
type WillMsg struct {
	Id int64 `gorm:"column:id" json:"id"`
	ClientId string `gorm:"column:client_id" json:"client_id"`
	PkId uint `gorm:"column:pk_id" json:"pk_id"`
	MTypeFlags int `gorm:"column:mtypeflags" json:"mtypeflags"`
	Topic string `gorm:"column:topic" json:"topic"`
	PayloadFormatIndicator int8 `gorm:"column:payload_format_indicator" json:"payload_format_indicator"`
	MsgExpiryInterval uint64 `gorm:"column:msg_expiry_interval" json:"msg_expiry_interval"`
	TopicAlias uint `gorm:"column:topic_alias" json:"topic_alias"`
	ResponseTopic string `gorm:"column:response_topic" json:"response_topic"`
	CorrelationData string `gorm:"column:correlation_data" json:"correlation_data"`
	UserProperty string `gorm:"column:user_property" json:"user_property"`
	SubIdentifier int64 `gorm:"column:sub_identifier" json:"sub_identifier"`
	ContentType string `gorm:"column:content_type" json:"content_type"`
	Payload string `gorm:"column:payload" json:"payload"`
	OfflineTime int64 `gorm:"column:offline_time" json:"offline_time"`
	WTime int64 `gorm:"column: wtime" json:" wtime"`
}
func (p WillMsg) TableName() string {
	return "willmsg"
}

type Session struct {
	Id uint64 `gorm:"column:id" json:"id"`
	ClientId string `gorm:"column:client_id" json:"client_id"`
	Status uint8 `gorm:"column:status" json:"status"`
	OfflineTime int64 `gorm:"column:offline_time" json:"offline_time"`
}
func (p Session) TableName() string {
	return "session"
}
