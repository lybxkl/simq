package message

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
	if this.header.remlen == 2 {
		this.reasonCode = Success
		return total, nil
	}
	return this.PubackMessage.decodeOther(src, total, n)
}
