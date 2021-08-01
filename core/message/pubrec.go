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
