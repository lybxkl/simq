package messagev5

import "fmt"

type PubrecMessage struct {
	PubackMessage
}

// A PUBREC Packet is the response to a PUBLISH Packet with QoS 2. It is the second
// packet of the QoS 2 protocol exchange.
var _ Message = (*PubrecMessage)(nil)

// NewPubrecMessage creates a new PUBREC message.
func NewPubrecMessage() *PubrecMessage {
	msg := &PubrecMessage{}
	msg.SetType(PUBREC)

	return msg
}
func (this *PubrecMessage) Decode(src []byte) (int, error) {
	total := 0

	n, err := this.header.decode(src[total:])
	total += n
	if err != nil {
		return total, err
	}
	//this.packetId = binary.BigEndian.Uint16(src[total:])
	this.packetId = src[total : total+2]
	total += 2
	if this.RemainingLength() == 2 {
		this.reasonCode = Success
		return total, nil
	}
	return this.decodeOther(src, total, n)
}
func (this *PubrecMessage) Encode(dst []byte) (int, error) {
	this.build()
	if !this.dirty {
		if len(dst) < len(this.dbuf) {
			return 0, fmt.Errorf("pubrec/Encode: Insufficient buffer size. Expecting %d, got %d.", len(this.dbuf), len(dst))
		}
		return copy(dst, this.dbuf), nil
	}

	hl := this.header.msglen()
	ml := this.msglen()

	if len(dst) < hl+ml {
		return 0, fmt.Errorf("pubrec/Encode: Insufficient buffer size. Expecting %d, got %d.", hl+ml, len(dst))
	}

	if err := this.SetRemainingLength(int32(ml)); err != nil {
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
	if !ValidPubRecReasonCode(this.reasonCode) {
		return total, ProtocolError
	}
	if this.propertyLen >= 4 {
		b := lbEncode(this.propertyLen)
		copy(dst[total:], b)
		total += len(b)
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
	for i := 0; i < len(this.userProperty); i++ {
		dst[total] = UserProperty
		total++
		n, err = writeLPBytes(dst[total:], this.userProperty[i])
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
