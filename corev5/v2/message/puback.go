package message

import (
	"bytes"
	"fmt"
)

// PubackMessage A PUBACK Packet is the response to a PUBLISH Packet with QoS level 1.
type PubackMessage struct {
	header
	// === PUBACK可变报头 ===
	reasonCode ReasonCode // 原因码
	//  属性长度
	propertyLen  uint32   // 如果剩余长度小于4字节，则没有属性长度
	reasonStr    []byte   // 原因字符串是为诊断而设计的可读字符串，不能被接收端所解析。如果加上原因字符串之后的PUBACK报文长度超出了接收端指定的最大报文长度（Maximum Packet Size），则发送端不能发送此原因字符串
	userProperty [][]byte // 用户属性如果加上原因字符串之后的PUBACK报文长度超出了接收端指定的最大报文长度（Maximum Packet Size），则发送端不能发送此用户属性
}

func (this *PubackMessage) ReasonCode() ReasonCode {
	return this.reasonCode
}

func (this *PubackMessage) SetReasonCode(reasonCode ReasonCode) {
	this.reasonCode = reasonCode
	this.dirty = true
}

func (this *PubackMessage) PropertyLen() uint32 {
	return this.propertyLen
}

func (this *PubackMessage) SetPropertyLen(propertyLen uint32) {
	this.propertyLen = propertyLen
	this.dirty = true
}

func (this *PubackMessage) ReasonStr() []byte {
	return this.reasonStr
}

func (this *PubackMessage) SetReasonStr(reasonStr []byte) {
	this.reasonStr = reasonStr
	this.dirty = true
}

func (this *PubackMessage) UserProperty() [][]byte {
	return this.userProperty
}

func (this *PubackMessage) AddUserPropertys(userProperty [][]byte) {
	this.userProperty = append(this.userProperty, userProperty...)
	this.dirty = true
}
func (this *PubackMessage) AddUserProperty(userProperty []byte) {
	this.userProperty = append(this.userProperty, userProperty)
	this.dirty = true
}

var _ Message = (*PubackMessage)(nil)

// NewPubackMessage creates a new PUBACK message.
func NewPubackMessage() *PubackMessage {
	msg := &PubackMessage{}
	msg.SetType(PUBACK)

	return msg
}

func (this PubackMessage) String() string {
	return fmt.Sprintf("%s, Packet ID=%d, Reason Code=%v, PropertyLen=%v, "+
		"Reason String=%s, User Property=%s",
		this.header, this.packetId, this.reasonCode, this.propertyLen, this.reasonStr, this.userProperty)
}

func (this *PubackMessage) Len() int {
	if !this.dirty {
		return len(this.dbuf)
	}

	ml := this.msglen()

	if err := this.SetRemainingLength(uint32(ml)); err != nil {
		return 0
	}

	return this.header.msglen() + ml
}

func (this *PubackMessage) Decode(src []byte) (int, error) {
	total := 0

	n, err := this.header.decode(src[total:])
	total += n
	if err != nil {
		return total, err
	}
	//this.packetId = binary.BigEndian.Uint16(src[total:])
	this.packetId = CopyLen(src[total:total+2], 2)
	total += 2
	if this.RemainingLength() == 2 {
		this.reasonCode = Success
		return total, nil
	}
	return this.decodeOther(src, total, n)
}

// 从可变包头中原因码开始处理
func (this *PubackMessage) decodeOther(src []byte, total, n int) (int, error) {
	var err error
	this.reasonCode = ReasonCode(src[total])
	total++
	if !ValidPubAckReasonCode(this.reasonCode) {
		return total, ProtocolError
	}
	if total < len(src) && len(src[total:]) > 0 {
		this.propertyLen, n, err = lbDecode(src[total:])
		total += n
		if err != nil {
			return total, err
		}
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
	this.dirty = false
	return total, nil
}

func (this *PubackMessage) Encode(dst []byte) (int, error) {
	if !this.dirty {
		if len(dst) < len(this.dbuf) {
			return 0, fmt.Errorf("pubxxx/Encode: Insufficient buffer size. Expecting %d, got %d.", len(this.dbuf), len(dst))
		}
		return copy(dst, this.dbuf), nil
	}

	ml := this.msglen()
	hl := this.header.msglen()

	if len(dst) < hl+ml {
		return 0, fmt.Errorf("pubxxx/Encode: Insufficient buffer size. Expecting %d, got %d.", hl+ml, len(dst))
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
	if this.header.remlen == 2 && this.reasonCode == Success {
		return total, nil
	}
	dst[total] = this.reasonCode.Value()
	total++
	if this.propertyLen > 0 {
		b := lbEncode(this.propertyLen)
		copy(dst[total:], b)
		total += len(b)
	} else {
		return total, nil
	}
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
	return total, nil
}

func (this *PubackMessage) EncodeToBuf(dst *bytes.Buffer) (int, error) {
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
	if len(this.packetId) == 2 {
		dst.Write(this.packetId)
	} else {
		dst.Write([]byte{0, 0})
	}

	if this.header.remlen == 2 && this.reasonCode == Success {
		return dst.Len(), nil
	}
	dst.WriteByte(this.reasonCode.Value())

	if this.propertyLen > 0 {
		dst.Write(lbEncode(this.propertyLen))
	} else {
		return dst.Len(), nil
	}
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
	return dst.Len(), nil
}

func (this *PubackMessage) build() {
	total := 0
	if len(this.reasonStr) > 0 {
		total++
		total += 2
		total += len(this.reasonStr)
	}

	n := buildUserPropertyLen(this.userProperty)
	total += n

	this.propertyLen = uint32(total)
	// 2 是报文标识符，2字节
	// 1 是原因码
	if this.propertyLen == 0 && this.reasonCode == Success {
		_ = this.SetRemainingLength(uint32(2))
		return
	}
	if this.propertyLen == 0 && this.reasonCode != Success {
		_ = this.SetRemainingLength(uint32(2 + 1))
		return
	}
	_ = this.SetRemainingLength(uint32(2 + 1 + int(this.propertyLen) + len(lbEncode(this.propertyLen))))
}
func (this *PubackMessage) msglen() int {
	this.build()
	return int(this.remlen)
}
