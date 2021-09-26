package messagev5

// PubcompMessage The PUBCOMP Packet is the response to a PUBREL Packet. It is the fourth and
// final packet of the QoS 2 protocol exchange.
type PubcompMessage struct {
	PubackMessage
}

var _ Message = (*PubcompMessage)(nil)

// NewPubcompMessage creates a new PUBCOMP message.
func NewPubcompMessage() *PubcompMessage {
	msg := &PubcompMessage{}
	msg.SetType(PUBCOMP)

	return msg
}

func (this *PubcompMessage) Decode(src []byte) (int, error) {
	total := 0

	n, err := this.header.decode(src[total:])
	total += n
	if err != nil {
		return total, err
	}
	//this.packetId = binary.BigEndian.Uint16(src[total:])
	this.packetId = CopyLen(src[total:total+2], 2)
	total += 2
	if this.header.remlen == 2 {
		this.reasonCode = Success
		return total, nil
	}
	return this.decodeOther(src, total, n)
}

// 从可变包头中原因码开始处理
func (this *PubcompMessage) decodeOther(src []byte, total, n int) (int, error) {
	var err error
	this.reasonCode = ReasonCode(src[total])
	total++
	if !ValidPubCompReasonCode(this.reasonCode) {
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
