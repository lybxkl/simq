package message

import "fmt"

// A PUBREL Packet is the response to a PUBREC Packet. It is the third packet of the
// QoS 2 protocol exchange.
type PubrelMessage struct {
	PubackMessage
}

var _ Message = (*PubrelMessage)(nil)

// NewPubrelMessage creates a new PUBREL message.
func NewPubrelMessage() *PubrelMessage {
	msg := &PubrelMessage{}
	msg.SetType(PUBREL)

	return msg
}
func (this *PubrelMessage) Decode(src []byte) (int, error) {
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

// 从可变包头中原因码开始处理
func (this *PubrelMessage) decodeOther(src []byte, total, n int) (int, error) {
	var err error
	this.reasonCode = ReasonCode(src[total])
	total++
	if !ValidPubRelReasonCode(this.reasonCode) {
		return total, ProtocolError
	}
	if total < len(src) && len(src[total:]) >= 4 {
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
	if total < len(src) && src[total] == UserProperty {
		total++
		this.userProperty = make([][]byte, 0)
		var uv []byte
		uv, n, err = readLPBytes(src[total:])
		total += n
		if err != nil {
			return total, err
		}
		this.userProperty = append(this.userProperty, uv)
		for total < len(src) && src[total] == UserProperty {
			total++
			uv, n, err = readLPBytes(src[total:])
			total += n
			if err != nil {
				return total, err
			}
			this.userProperty = append(this.userProperty, uv)
		}
	}
	this.dirty = false
	return total, nil
}
func (this *PubrelMessage) Encode(dst []byte) (int, error) {
	this.build()
	if !this.dirty {
		if len(dst) < len(this.dbuf) {
			return 0, fmt.Errorf("pubrel/Encode: Insufficient buffer size. Expecting %d, got %d.", len(this.dbuf), len(dst))
		}
		return copy(dst, this.dbuf), nil
	}

	hl := this.header.msglen()
	ml := this.msglen()

	if len(dst) < hl+ml {
		return 0, fmt.Errorf("pubrel/Encode: Insufficient buffer size. Expecting %d, got %d.", hl+ml, len(dst))
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
