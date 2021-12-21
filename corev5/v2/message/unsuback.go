package message

import (
	"bytes"
	"fmt"
)

// UnsubackMessage The UNSUBACK Packet is sent by the Server to the Client to confirm receipt of an
// UNSUBSCRIBE Packet.
type UnsubackMessage struct {
	header
	//  属性长度
	propertyLen  uint32   // 如果剩余长度小于4字节，则没有属性长度
	reasonStr    []byte   // 原因字符串是为诊断而设计的可读字符串，不能被接收端所解析。如果加上原因字符串之后的PUBACK报文长度超出了接收端指定的最大报文长度（Maximum Packet Size），则发送端不能发送此原因字符串
	userProperty [][]byte // 用户属性如果加上原因字符串之后的PUBACK报文长度超出了接收端指定的最大报文长度（Maximum Packet Size），则发送端不能发送此用户属性
	reasonCodes  []byte
}

func (u *UnsubackMessage) PropertyLen() uint32 {
	return u.propertyLen
}

func (u *UnsubackMessage) SetPropertyLen(propertyLen uint32) {
	u.propertyLen = propertyLen
	u.dirty = true
}

func (u *UnsubackMessage) ReasonStr() []byte {
	return u.reasonStr
}

func (u *UnsubackMessage) SetReasonStr(reasonStr []byte) {
	u.reasonStr = reasonStr
	u.dirty = true
}

func (u *UnsubackMessage) UserProperty() [][]byte {
	return u.userProperty
}

func (this *UnsubackMessage) AddUserPropertys(userProperty [][]byte) {
	this.userProperty = append(this.userProperty, userProperty...)
	this.dirty = true
}
func (this *UnsubackMessage) AddUserProperty(userProperty []byte) {
	this.userProperty = append(this.userProperty, userProperty)
	this.dirty = true
}

func (u *UnsubackMessage) ReasonCodes() []byte {
	return u.reasonCodes
}

func (u *UnsubackMessage) AddReasonCodes(reasonCodes []byte) {
	u.reasonCodes = append(u.reasonCodes, reasonCodes...)
	u.dirty = true
}
func (u *UnsubackMessage) AddReasonCode(reasonCode byte) {
	u.reasonCodes = append(u.reasonCodes, reasonCode)
	u.dirty = true
}

var _ Message = (*UnsubackMessage)(nil)

// NewUnsubackMessage creates a new UNSUBACK message.
func NewUnsubackMessage() *UnsubackMessage {
	msg := &UnsubackMessage{}
	msg.SetType(UNSUBACK)

	return msg
}
func (this UnsubackMessage) String() string {
	return fmt.Sprintf("%s, Packet ID=%d, PropertyLen=%v, "+
		"Reason String=%s, User Property=%s, Reason Code=%v",
		this.header, this.packetId, this.propertyLen, this.reasonStr, this.userProperty, this.reasonCodes)
}
func (this *UnsubackMessage) Decode(src []byte) (int, error) {
	total, n := 0, 0

	hn, err := this.header.decode(src[total:])
	total += hn
	n = hn
	if err != nil {
		return total, err
	}
	//this.packetId = binary.BigEndian.Uint16(src[total:])
	this.packetId = CopyLen(src[total:total+2], 2)
	total += 2

	this.propertyLen, n, err = lbDecode(src[total:])
	total += n
	if err != nil {
		return total, err
	}

	if total < len(src) && src[total] == ReasonString {
		total++
		this.reasonStr, n, err = readLPBytes(src[total:])
		total += n
		if err != nil {
			return total, err
		}
		if src[total] == ReasonString {
			return total, ProtocolError
		}
	}

	this.userProperty, n, err = decodeUserProperty(src[total:]) // 用户属性
	total += n
	if err != nil {
		return total, err
	}

	l := int(this.remlen) - (total - hn)
	if l == 0 {
		return total, ProtocolError
	}
	this.reasonCodes = CopyLen(src[total:total+l], l)
	total += len(this.reasonCodes)

	for _, code := range this.reasonCodes {
		if !ValidUnSubAckReasonCode(ReasonCode(code)) {
			return total, ProtocolError // fmt.Errorf("unsuback/Decode: Invalid return code %d for topic %d", code, i)
		}
	}

	this.dirty = false
	return total, nil
}

func (this *UnsubackMessage) Encode(dst []byte) (int, error) {
	if !this.dirty {
		if len(dst) < len(this.dbuf) {
			return 0, fmt.Errorf("unsuback/Encode: Insufficient buffer size. Expecting %d, got %d.", len(this.dbuf), len(dst))
		}
		return copy(dst, this.dbuf), nil
	}

	ml := this.msglen()
	hl := this.header.msglen()

	if len(dst) < hl+ml {
		return 0, fmt.Errorf("unsuback/Encode: Insufficient buffer size. Expecting %d, got %d.", hl+ml, len(dst))
	}

	if err := this.SetRemainingLength(uint32(ml)); err != nil {
		return 0, err
	}

	total := 0

	n, err := this.header.encode(dst[total:])
	total += n
	if err != nil {
		return total, err
	}

	// 可变报头
	if copy(dst[total:total+2], this.packetId) != 2 {
		dst[total], dst[total+1] = 0, 0
	}
	total += 2

	b := lbEncode(this.propertyLen)
	copy(dst[total:], b)
	total += len(b)

	// TODO 下面两个在PUBACK报文长度超出了接收端指定的最大报文长度（Maximum Packet Size），则发送端不能发送此原因字符串
	if len(this.reasonStr) > 0 {
		dst[total] = ReasonString
		total++
		n, err = writeLPBytes(dst[total:], this.reasonStr)
		total += n
		if err != nil {
			return total, err
		}
	}

	n, err = writeUserProperty(dst[total:], this.userProperty) // 用户属性
	total += n

	copy(dst[total:], this.reasonCodes)
	total += len(this.reasonCodes)

	return total, nil
}

func (this *UnsubackMessage) EncodeToBuf(dst *bytes.Buffer) (int, error) {
	if !this.dirty {
		return dst.Write(this.dbuf)
	}

	ml := this.msglen()

	if err := this.SetRemainingLength(uint32(ml)); err != nil {
		return 0, err
	}

	_, err := this.header.encodeToBuf(dst)
	if err != nil {
		return dst.Len(), err
	}

	// 可变报头
	if len(this.packetId) != 2 {
		dst.Write([]byte{0, 0})
	} else {
		dst.Write(this.packetId)
	}

	dst.Write(lbEncode(this.propertyLen))

	// TODO 下面两个在PUBACK报文长度超出了接收端指定的最大报文长度（Maximum Packet Size），则发送端不能发送此原因字符串
	if len(this.reasonStr) > 0 {
		dst.WriteByte(ReasonString)
		_, err = writeToBufLPBytes(dst, this.reasonStr)
		if err != nil {
			return dst.Len(), err
		}
	}

	_, err = writeUserPropertyByBuf(dst, this.userProperty) // 用户属性
	if err != nil {
		return dst.Len(), err
	}

	dst.Write(this.reasonCodes)
	return dst.Len(), nil
}

func (this *UnsubackMessage) build() {
	// packet ID
	total := 2

	if len(this.reasonStr) != 0 {
		total++
		total += 2
		total += len(this.reasonStr)
	}

	n := buildUserPropertyLen(this.userProperty) // 用户属性
	total += n

	this.propertyLen = uint32(total - 2)
	total += len(lbEncode(this.propertyLen))
	total += len(this.reasonCodes)
	_ = this.SetRemainingLength(uint32(total))
}
func (this *UnsubackMessage) msglen() int {
	this.build()
	return int(this.remlen)
}

func (this *UnsubackMessage) Len() int {
	if !this.dirty {
		return len(this.dbuf)
	}

	ml := this.msglen()

	if err := this.SetRemainingLength(uint32(ml)); err != nil {
		return 0
	}

	return this.header.msglen() + ml
}
