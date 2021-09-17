package sessionsv5

import (
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"gitee.com/Ljolan/si-mqtt/corev5/topicsv5"
)

type SessionInitTopic topicsv5.Sub
type Session interface {
	Init(msg *messagev5.ConnectMessage, topics ...SessionInitTopic) error
	Update(msg *messagev5.ConnectMessage) error
	OfflineMsg() []messagev5.Message

	AddTopic(sub topicsv5.Sub) error
	RemoveTopic(topic string) error
	Topics() ([]topicsv5.Sub, error)
	SubOption(topic []byte) topicsv5.Sub // 获取主题的订阅选项

	ID() string  // 客户端id
	IDs() []byte // 客户端id 字节类型的

	Cmsg() *messagev5.ConnectMessage
	Will() *messagev5.PublishMessage

	Pub1ack() Ackqueue
	Pub2in() Ackqueue
	Pub2out() Ackqueue
	Suback() Ackqueue
	Unsuback() Ackqueue
	Pingack() Ackqueue
	SessionExpand
}

type SessionExpand interface {
	ExpiryInterval() uint32
	Status() Status
	ClientId() string
	ReceiveMaximum() uint16
	MaxPacketSize() uint32
	TopicAliasMax() uint16
	RequestRespInfo() byte
	RequestProblemInfo() byte
	UserProperty() []string

	OfflineTime() int64

	SetExpiryInterval(uint32)
	SetStatus(Status)
	SetClientId(string)
	SetReceiveMaximum(uint16)
	SetMaxPacketSize(uint32)
	SetTopicAliasMax(uint16)
	SetRequestRespInfo(byte)
	SetRequestProblemInfo(byte)
	SetUserProperty([]string)

	SetOfflineTime(int64)

	SetWill(*messagev5.PublishMessage)
	SetSub(*messagev5.SubscribeMessage)
}

type Status uint8

const (
	_       Status = iota
	NULL           // 从未连接过（之前 cleanStart为1 的也为NULL）
	ONLINE         // 在线
	OFFLINE        // cleanStart为0，且连接过mqtt集群，已离线，会返回offlineTime（离线时间）
)
