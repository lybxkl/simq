package po

type Subscription struct {
	ClientId        string `gorm:"column:client_id;index:cid"`
	Qos             uint8  `gorm:"column:qos"`
	Topic           string `gorm:"column:topic;index:topic"`
	SubId           uint32 `gorm:"column:sub_id"` // 订阅标识符
	NoLocal         bool   `gorm:"column:no_local"`
	RetainAsPublish bool   `gorm:"column:retain_as_publish"`
	RetainHandling  uint8  `gorm:"column:retain_handling"`
}

func (w Subscription) TableName() string {
	return "si_sub"
}
