package messagev5

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
	this.packetId = CopyLen(src[total:total+2], 2)
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
