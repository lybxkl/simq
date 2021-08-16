package message

// The PUBCOMP Packet is the response to a PUBREL Packet. It is the fourth and
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
	this.packetId = src[total : total+2]
	total += 2
	if this.header.remlen == 2 {
		this.reasonCode = Success
		return total, nil
	}
	return this.PubackMessage.decodeOther(src, total, n)
}
