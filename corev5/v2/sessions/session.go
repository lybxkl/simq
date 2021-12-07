package sessions

import (
	"gitee.com/Ljolan/si-mqtt/corev5/v2/message"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/topics"
)

type SessionInitTopic topics.Sub

type Session interface {
	Init(msg *message.ConnectMessage, topics ...SessionInitTopic) error
	Update(msg *message.ConnectMessage) error
	OfflineMsg() []message.Message

	AddTopic(sub topics.Sub) error
	AddTopicAlice(topic []byte, alice uint16)
	GetTopicAlice(topic []byte) (uint16, bool)
	GetAliceTopic(alice uint16) ([]byte, bool)

	RemoveTopic(topic string) error
	Topics() ([]topics.Sub, error)
	SubOption(topic []byte) topics.Sub // 获取主题的订阅选项

	ID() string  // 客户端id
	IDs() []byte // 客户端id 字节类型的

	Cmsg() *message.ConnectMessage
	Will() *message.PublishMessage

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

	SetWill(*message.PublishMessage)
	SetSub(*message.SubscribeMessage)
}

type Status uint8

const (
	_       Status = iota
	NULL           // 从未连接过（之前 cleanStart为1 的也为NULL）
	ONLINE         // 在线
	OFFLINE        // cleanStart为0，且连接过mqtt集群，已离线，会返回offlineTime（离线时间）
)
