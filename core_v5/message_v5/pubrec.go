package message

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
	if this.header.remlen == 2 {
		this.reasonCode = Success
		return total, nil
	}
	return this.PubackMessage.decodeOther(src, total, n)
}
