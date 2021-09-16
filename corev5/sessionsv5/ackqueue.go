package sessionsv5

import "gitee.com/Ljolan/si-mqtt/corev5/messagev5"

type Ackqueue interface {
	Wait(msg messagev5.Message, onComplete interface{}) error
	Ack(msg messagev5.Message) error
	Acked() []Ackmsg
	Size() int64
	Len() int
}

type Ackmsg struct {
	// Message type of the messagev5 waiting for ack
	//等待ack消息的消息类型
	Mtype messagev5.MessageType

	// Current state of the ack-waiting messagev5
	//等待ack-waiting消息的当前状态
	State messagev5.MessageType

	// Packet ID of the messagev5. Every messagev5 that require ack'ing must have a valid
	// packet ID. Messages that have messagev5 I
	//消息的包ID。每个需要ack'ing的消息必须有一个有效的
	//数据包ID，包含消息I的消息
	Pktid uint16

	// Slice containing the messagev5 bytes
	//包含消息字节的片
	Msgbuf []byte

	// Slice containing the ack messagev5 bytes
	//包含ack消息字节的片
	Ackbuf []byte

	// When ack cycle completes, call this function
	//当ack循环完成时，调用这个函数
	OnComplete interface{}
}
