package messagev5

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
	this.packetId = CopyLen(src[total:total+2], 2)
	total += 2
	if this.RemainingLength() == 2 {
		this.reasonCode = Success
		return total, nil
	}
	return this.decodeOther(src, total, n)
}

// 从可变包头中原因码开始处理
func (this *PubrecMessage) decodeOther(src []byte, total, n int) (int, error) {
	var err error
	this.reasonCode = ReasonCode(src[total])
	total++
	if !ValidPubRecReasonCode(this.reasonCode) {
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
