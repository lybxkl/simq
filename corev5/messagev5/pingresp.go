package messagev5

import "fmt"

// A PINGRESP Packet is sent by the Server to the Client in response to a PINGREQ
// Packet. It indicates that the Server is alive.
type PingrespMessage struct {
	header
}

var _ Message = (*PingrespMessage)(nil)

// NewPingrespMessage creates a new PINGRESP message.
func NewPingrespMessage() *PingrespMessage {
	msg := &PingrespMessage{}
	msg.SetType(PINGRESP)

	return msg
}
func (this *PingrespMessage) Decode(src []byte) (int, error) {
	return this.header.decode(src)
}

func (this *PingrespMessage) Encode(dst []byte) (int, error) {
	if !this.dirty {
		if len(dst) < len(this.dbuf) {
			return 0, fmt.Errorf("pingrespMessage/Encode: Insufficient buffer size. Expecting %d, got %d.", len(this.dbuf), len(dst))
		}

		return copy(dst, this.dbuf), nil
	}

	return this.header.encode(dst)
}
