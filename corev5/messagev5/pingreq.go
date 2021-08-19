package messagev5

import "fmt"

// The PINGREQ Packet is sent from a Client to the Server. It can be used to:
// 1. Indicate to the Server that the Client is alive in the absence of any other
//    Control Packets being sent from the Client to the Server.
// 2. Request that the Server responds to confirm that it is alive.
// 3. Exercise the network to indicate that the Network Connection is active.
type PingreqMessage struct {
	header
}

var _ Message = (*PingreqMessage)(nil)

// NewPingreqMessage creates a new PINGREQ message.
func NewPingreqMessage() *PingreqMessage {
	msg := &PingreqMessage{}
	msg.SetType(PINGREQ)

	return msg
}

func (this *PingreqMessage) Decode(src []byte) (int, error) {
	return this.header.decode(src)
}

func (this *PingreqMessage) Encode(dst []byte) (int, error) {
	if !this.dirty {
		if len(dst) < len(this.dbuf) {
			return 0, fmt.Errorf("pingreqMessage/Encode: Insufficient buffer size. Expecting %d, got %d.", len(this.dbuf), len(dst))
		}

		return copy(dst, this.dbuf), nil
	}

	return this.header.encode(dst)
}
