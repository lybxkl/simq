package messagev5

import (
	"encoding/binary"
	"fmt"
)

var (
	gPacketId uint64 = 0
)

// Support v5
// 固定头
// - 1字节的控制包类型(位7-4)和标志(位3-0)
// -最多4字节的剩余长度
type header struct {
	// 剩余长度 变长字节整数, 用来表示当前控制报文剩余部分的字节数, 包括可变报头和负载的数据.
	// 剩余长度不包括用于编码剩余长度字段本身的字节数.
	// MQTT控制报文总长度等于固定报头的长度加上剩余长度.
	remlen int32

	// mtypeflags是固定报头的第一个字节，前四位4位是mtype, 4位是flags标志
	mtypeflags []byte

	// Some messages need packet ID, 2 byte uint16
	//一些消息需要数据包ID, 2字节uint16
	// 也就是报文标识符，两个字节
	// 需要这个的有以下报文类型
	// PUBLISH报文（当QoS>0时），PUBACK，PUBREC，PUBREC，PUBREL，PUBCOMP，SUBSCRIBE，SUBACK，UNSUBSCRIBE，UNSUBACK
	// 当客户端处理完这个报文对应的确认后，这个报文标识符就释放可重用
	packetId []byte

	// Points to the decoding buffer
	//指向解码缓冲区
	dbuf []byte

	// Whether the message has changed since last decode
	//自上次解码以来，消息是否发生了变化
	dirty bool
}

// String returns a string representation of the message.
func (this header) String() string {
	return fmt.Sprintf("Type=%q, Flags=%04b, Remaining Length=%d, Packet Id=%v", this.Type().Name(), this.Flags(), this.remlen, this.packetId)
}

// Name returns a string representation of the message type. Examples include
// "PUBLISH", "SUBSCRIBE", and others. This is statically defined for each of
// the message types and cannot be changed.
func (this *header) Name() string {
	return this.Type().Name()
}
func (this *header) MtypeFlags() byte {
	if this.mtypeflags == nil {
		return 0
	}
	return this.mtypeflags[0]
}
func (this *header) SetMtypeFlags(mtypeflags byte) {
	if len(this.mtypeflags) != 1 {
		this.mtypeflags = make([]byte, 1)
	}
	this.mtypeflags[0] = mtypeflags
}

// Desc returns a string description of the message type. For example, a
// CONNECT message would return "Client request to connect to Server." These
// descriptions are statically defined (copied from the MQTT spec) and cannot
// be changed.
func (this *header) Desc() string {
	return this.Type().Desc()
}

// Type returns the MessageType of the Message. The retured value should be one
// of the constants defined for MessageType.
func (this *header) Type() MessageType {
	//return this.mtype
	if len(this.mtypeflags) != 1 {
		this.mtypeflags = make([]byte, 1)
		this.dirty = true
	}

	return MessageType(this.mtypeflags[0] >> 4)
}

// SetType sets the message type of this message. It also correctly sets the
// default flags for the message type. It returns an error if the type is invalid.
func (this *header) SetType(mtype MessageType) error {
	if !mtype.Valid() {
		return fmt.Errorf("header/SetType: Invalid control packet type %d", mtype)
	}

	// Notice we don't set the message to be dirty when we are not allocating a new
	// buffer. In this case, it means the buffer is probably a sub-slice of another
	// slice. If that's the case, then during encoding we would have copied the whole
	// backing buffer anyway.
	if len(this.mtypeflags) != 1 {
		this.mtypeflags = make([]byte, 1)
		this.dirty = true
	}

	this.mtypeflags[0] = byte(mtype)<<4 | (mtype.DefaultFlags() & 0xf)

	return nil
}

// Flags returns the fixed header flags for this message.
func (this *header) Flags() byte {
	//return this.flags
	return this.mtypeflags[0] & 0x0f
}

// RemainingLength returns the length of the non-fixed-header part of the message.
func (this *header) RemainingLength() int32 {
	return this.remlen
}

// SetRemainingLength sets the length of the non-fixed-header part of the message.
// It returns error if the length is greater than 268435455, which is the max
// message length as defined by the MQTT spec.
func (this *header) SetRemainingLength(remlen int32) error {
	if remlen > maxRemainingLength || remlen < 0 {
		return fmt.Errorf("header/SetLength: Remaining length (%d) out of bound (max %d, min 0)", remlen, maxRemainingLength)
	}

	this.remlen = remlen
	this.dirty = true

	return nil
}

func (this *header) Len() int {
	return this.msglen()
}

// PacketId returns the ID of the packet.
func (this *header) PacketId() uint16 {
	if len(this.packetId) == 2 {
		return binary.BigEndian.Uint16(this.packetId)
	}

	return 0
}

// SetPacketId sets the ID of the packet.
func (this *header) SetPacketId(v uint16) {
	// If setting to 0, nothing to do, move on
	if v == 0 {
		return
	}

	// If packetId buffer is not 2 bytes (uint16), then we allocate a new one and
	// make dirty. Then we encode the packet ID into the buffer.
	if len(this.packetId) != 2 {
		this.packetId = make([]byte, 2)
		this.dirty = true
	}

	// Notice we don't set the message to be dirty when we are not allocating a new
	// buffer. In this case, it means the buffer is probably a sub-slice of another
	// slice. If that's the case, then during encoding we would have copied the whole
	// backing buffer anyway.
	binary.BigEndian.PutUint16(this.packetId, v)
}

func (this *header) encode(dst []byte) (int, error) {
	ml := this.msglen()

	if len(dst) < ml {
		return 0, fmt.Errorf("header/Encode: Insufficient buffer size. Expecting %d, got %d.", ml, len(dst))
	}

	total := 0

	if this.remlen > maxRemainingLength || this.remlen < 0 {
		return total, fmt.Errorf("header/Encode: Remaining length (%d) out of bound (max %d, min 0)", this.remlen, maxRemainingLength)
	}

	if !this.Type().Valid() {
		return total, fmt.Errorf("header/Encode: Invalid message type %d", this.Type())
	}

	dst[total] = this.mtypeflags[0]
	total += 1

	n := binary.PutUvarint(dst[total:], uint64(this.remlen))
	total += n

	return total, nil
}

// Decode reads from the io.Reader parameter until a full message is decoded, or
// when io.Reader returns EOF or error. The first return value is the number of
// bytes read from io.Reader. The second is error if Decode encounters any problems.
func (this *header) decode(src []byte) (int, error) {
	total := 0

	this.dbuf = src

	mtype := this.Type()
	//mtype := MessageType(0)
	this.mtypeflags = CopyLen(src[total:total+1], 1) //src[total : total+1]
	//mtype := MessageType(src[total] >> 4)
	if !this.Type().Valid() {
		return total, InvalidMessage //fmt.Errorf("header/Decode: Invalid message type %d.", mtype)
	}

	if mtype != this.Type() {
		return total, InvalidMessage //fmt.Errorf("header/Decode: Invalid message type %d. Expecting %d.", this.Type(), mtype)
	}
	//this.flags = src[total] & 0x0f
	if this.Type() != PUBLISH && this.Flags() != this.Type().DefaultFlags() {
		return total, InvalidMessage //fmt.Errorf("header/Decode: Invalid message (%d) flags. Expecting %d, got %d", this.Type(), this.Type().DefaultFlags(), this.Flags())
	}
	// Bit 3	Bit 2	Bit 1	 Bit 0
	// DUP	         QOS	     RETAIN
	// publish 报文，验证qos，第一个字节的第1，2位
	if this.Type() == PUBLISH && !ValidQos((this.Flags()>>1)&0x3) {
		return total, InvalidMessage //fmt.Errorf("header/Decode: Invalid QoS (%d) for PUBLISH message.", (this.Flags()>>1)&0x3)
	}

	total++ // 第一个字节处理完毕

	// 剩余长度，传进来的就是只有固定报头数据，所以剩下的就是剩余长度变长解码的数据
	remlen, m := binary.Uvarint(src[total:])
	total += m
	this.remlen = int32(remlen)

	if this.remlen > maxRemainingLength ||
		(this.Type() != PINGREQ && this.Type() != PINGRESP &&
			this.Type() != AUTH && this.Type() != DISCONNECT && remlen == 0) {
		return total, InvalidMessage //fmt.Errorf("header/Decode: Remaining length (%d) out of bound (max %d, min 0)", this.remlen, maxRemainingLength)
	}

	if int(this.remlen) > len(src[total:]) {
		return total, InvalidMessage //fmt.Errorf("header/Decode: Remaining length (%d) is greater than remaining buffer (%d)", this.remlen, len(src[total:]))
	}

	return total, nil
}

func (this *header) msglen() int {
	// message type and flag byte
	total := 1

	if this.remlen <= 127 {
		total += 1
	} else if this.remlen <= 16383 {
		total += 2
	} else if this.remlen <= 2097151 {
		total += 3
	} else {
		total += 4
	}

	return total
}
