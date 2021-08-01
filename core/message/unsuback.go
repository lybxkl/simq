package message

// The UNSUBACK Packet is sent by the Server to the Client to confirm receipt of an
// UNSUBSCRIBE Packet.
type UnsubackMessage struct {
	PubackMessage
}

var _ Message = (*UnsubackMessage)(nil)

// NewUnsubackMessage creates a new UNSUBACK message.
func NewUnsubackMessage() *UnsubackMessage {
	msg := &UnsubackMessage{}
	msg.SetType(UNSUBACK)

	return msg
}
