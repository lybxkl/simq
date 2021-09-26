package messagev5

import "fmt"

// SubackMessage A SUBACK Packet is sent by the Server to the Client to confirm receipt and processing
// of a SUBSCRIBE Packet.
//
// A SUBACK Packet contains a list of return codes, that specify the maximum QoS level
// that was granted in each Subscription that was requested by the SUBSCRIBE.
type SubackMessage struct {
	header
	// 可变报头
	propertiesLen uint32 // 属性长度
	reasonStr     []byte // 原因字符串
	userProperty  [][]byte
	// 载荷
	reasonCodes []byte
}

func (this *SubackMessage) ReasonStr() []byte {
	return this.reasonStr
}

func (this *SubackMessage) SetReasonStr(reasonStr []byte) {
	this.reasonStr = reasonStr
	this.dirty = true
}

var _ Message = (*SubackMessage)(nil)

// NewSubackMessage creates a new SUBACK message.
func NewSubackMessage() *SubackMessage {
	msg := &SubackMessage{}
	msg.SetType(SUBACK)

	return msg
}

// String returns a string representation of the message.
func (this SubackMessage) String() string {
	return fmt.Sprintf("%s, Packet ID=%d, Return Codes=%v, ", this.header, this.PacketId(), this.reasonCodes) +
		fmt.Sprintf("PropertiesLen=%v, Reason Str=%v, User Properties=%v", this.PropertiesLen(), this.ReasonStr(), this.UserProperty()) +
		fmt.Sprintf("%v", this.reasonCodes)

}
func (this *SubackMessage) PropertiesLen() uint32 {
	return this.propertiesLen
}

func (this *SubackMessage) SetPropertiesLen(propertiesLen uint32) {
	this.propertiesLen = propertiesLen
	this.dirty = true
}

func (this *SubackMessage) UserProperty() [][]byte {
	return this.userProperty
}

func (this *SubackMessage) AddUserPropertys(userProperty [][]byte) {
	this.userProperty = append(this.userProperty, userProperty...)
	this.dirty = true
}
func (this *SubackMessage) AddUserProperty(userProperty []byte) {
	this.userProperty = append(this.userProperty, userProperty)
	this.dirty = true
}

// ReasonCodes returns the list of QoS returns from the subscriptions sent in the SUBSCRIBE message.
func (this *SubackMessage) ReasonCodes() []byte {
	return this.reasonCodes
}

// AddreasonCodes sets the list of QoS returns from the subscriptions sent in the SUBSCRIBE message.
// An error is returned if any of the QoS values are not valid.
func (this *SubackMessage) AddReasonCodes(ret []byte) error {
	for _, c := range ret {
		if c != QosAtMostOnce && c != QosAtLeastOnce && c != QosExactlyOnce && c != QosFailure {
			return fmt.Errorf("suback/AddReturnCode: Invalid return code %d. Must be 0, 1, 2, 0x80.", c)
		}

		this.reasonCodes = append(this.reasonCodes, c)
	}

	this.dirty = true

	return nil
}

// AddReturnCode adds a single QoS return value.
func (this *SubackMessage) AddReasonCode(ret byte) error {
	return this.AddReasonCodes([]byte{ret})
}

func (this *SubackMessage) Len() int {
	if !this.dirty {
		return len(this.dbuf)
	}

	ml := this.msglen()

	if err := this.SetRemainingLength(int32(ml)); err != nil {
		return 0
	}

	return this.header.msglen() + ml
}

func (this *SubackMessage) Decode(src []byte) (int, error) {
	total := 0

	hn, err := this.header.decode(src[total:])
	total += hn
	if err != nil {
		return total, err
	}

	//this.packetId = binary.BigEndian.Uint16(src[total:])
	this.packetId = CopyLen(src[total:total+2], 2)
	total += 2
	var n int
	this.propertiesLen, n, err = lbDecode(src[total:])
	total += n
	if err != nil {
		return total, err
	}
	if int(this.propertiesLen) > len(src[total:]) {
		return total, ProtocolError
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
		var tb []byte
		this.userProperty = make([][]byte, 0)
		tb, n, err = readLPBytes(src[total:])
		total += n
		if err != nil {
			return total, err
		}
		this.userProperty = append(this.userProperty, tb)
		for total < len(src) && src[total] == UserProperty {
			total++
			tb, n, err = readLPBytes(src[total:])
			total += n
			if err != nil {
				return total, err
			}
			this.userProperty = append(this.userProperty, tb)
		}
	}

	l := int(this.remlen) - (total - hn)
	if l == 0 {
		return total, ProtocolError
	}
	this.reasonCodes = CopyLen(src[total:total+l], l)
	total += len(this.reasonCodes)

	for _, code := range this.reasonCodes {
		if !ValidSubAckReasonCode(ReasonCode(code)) {
			return total, ProtocolError // fmt.Errorf("suback/Decode: Invalid return code %d for topic %d", code, i)
		}
	}

	this.dirty = false

	return total, nil
}

func (this *SubackMessage) Encode(dst []byte) (int, error) {
	if !this.dirty {
		if len(dst) < len(this.dbuf) {
			return 0, fmt.Errorf("suback/Encode: Insufficient buffer size. Expecting %d, got %d.", len(this.dbuf), len(dst))
		}

		return copy(dst, this.dbuf), nil
	}

	for i, code := range this.reasonCodes {
		if !ValidSubAckReasonCode(ReasonCode(code)) {
			return 0, fmt.Errorf("suback/Encode: Invalid return code %d for topic %d", code, i)
		}
	}

	ml := this.msglen()
	hl := this.header.msglen()

	if len(dst) < hl+ml {
		return 0, fmt.Errorf("suback/Encode: Insufficient buffer size. Expecting %d, got %d.", hl+ml, len(dst))
	}

	if err := this.SetRemainingLength(int32(ml)); err != nil {
		return 0, err
	}

	total := 0

	n, err := this.header.encode(dst[total:])
	total += n
	if err != nil {
		return total, err
	}

	if copy(dst[total:total+2], this.packetId) != 2 {
		dst[total], dst[total+1] = 0, 0
	}
	total += 2

	tb := lbEncode(this.propertiesLen)
	copy(dst[total:], tb)
	total += len(tb)

	if len(this.reasonStr) > 0 { // todo && 太长，超过了客户端指定的最大报文长度，不发送
		dst[total] = ReasonString
		total++
		n, err = writeLPBytes(dst[total:], this.reasonStr)
		total += n
		if err != nil {
			return total, err
		}
	}
	if len(this.userProperty) > 0 {
		for i := 0; i < len(this.userProperty); i++ {
			dst[total] = UserProperty
			total++
			n, err = writeLPBytes(dst[total:], this.userProperty[i])
			total += n
			if err != nil {
				return total, err
			}
		}
	}

	copy(dst[total:], this.reasonCodes)
	total += len(this.reasonCodes)

	return total, nil
}
func (this *SubackMessage) build() {
	// packet ID
	total := 2

	if len(this.reasonStr) > 0 { // todo 最大长度时
		total++
		total += 2
		total += len(this.reasonStr)
	}
	for i := 0; i < len(this.userProperty); i++ {
		total++
		total += 2
		total += len(this.userProperty[i])
	}
	this.propertiesLen = uint32(total - 2)
	total += len(lbEncode(this.propertiesLen))
	total += len(this.reasonCodes)
	_ = this.SetRemainingLength(int32(total))
}
func (this *SubackMessage) msglen() int {
	this.build()

	return int(this.remlen)
}
