package po

type Subscription struct {
	ClientId        string `bson:"client_id,index"`
	Qos             uint8  `bson:"qos"`
	Topic           string `bson:"topic,index"`
	SubId           uint32 `bson:"sub_id"` // 订阅标识符
	NoLocal         bool   `bson:"no_local"`
	RetainAsPublish bool   `bson:"retain_as_publish"`
	RetainHandling  uint8  `bson:"retain_handling"`
}
