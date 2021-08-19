package message

import (
	"bytes"
	"fmt"
	"sync/atomic"
)

// SubscribeMessage The SUBSCRIBE Packet is sent from the Client to the Server to create one or more
// Subscriptions. Each Subscription registers a Client’s interest in one or more
// Topics. The Server sends PUBLISH Packets to the Client in order to forward
// Application Messages that were published to Topics that match these Subscriptions.
// The SUBSCRIBE Packet also specifies (for each Subscription) the maximum QoS with
// which the Server can send Application Messages to the Client.
type SubscribeMessage struct {
	header
	// 可变报头
	propertiesLen          uint32 // 属性长度
	subscriptionIdentifier uint32 // 订阅标识符 变长字节整数 取值范围从1到268,435,455
	userProperty           [][]byte
	// 载荷

	topics [][]byte
	qos    []byte
}

func (this *SubscribeMessage) PropertiesLen() uint32 {
	return this.propertiesLen
}

func (this *SubscribeMessage) SetPropertiesLen(propertiesLen uint32) {
	this.propertiesLen = propertiesLen
	this.dirty = true
}

func (this *SubscribeMessage) SubscriptionIdentifier() uint32 {
	return this.subscriptionIdentifier
}

func (this *SubscribeMessage) SetSubscriptionIdentifier(subscriptionIdentifier uint32) {
	this.subscriptionIdentifier = subscriptionIdentifier
	this.dirty = true
}

func (this *SubscribeMessage) UserProperty() [][]byte {
	return this.userProperty
}

func (this *SubscribeMessage) AddUserPropertys(userProperty [][]byte) {
	this.userProperty = append(this.userProperty, userProperty...)
	this.dirty = true
}
func (this *SubscribeMessage) AddUserProperty(userProperty []byte) {
	this.userProperty = append(this.userProperty, userProperty)
	this.dirty = true
}

var _ Message = (*SubscribeMessage)(nil)

// NewSubscribeMessage creates a new SUBSCRIBE message.
func NewSubscribeMessage() *SubscribeMessage {
	msg := &SubscribeMessage{}
	msg.SetType(SUBSCRIBE)

	return msg
}

func (this SubscribeMessage) String() string {
	msgstr := fmt.Sprintf("%s, Packet ID=%d", this.header, this.PacketId())
	msgstr = fmt.Sprintf("%s, PropertiesLen=%v, Subscription Identifier=%v, User Properties=%v", msgstr, this.PropertiesLen(), this.subscriptionIdentifier, this.UserProperty())

	for i, t := range this.topics {
		msgstr = fmt.Sprintf("%s, Topic[%d]=%q/%d", msgstr, i, string(t), this.qos[i])
	}

	return msgstr + "\n"
}

// Topics returns a list of topics sent by the Client.
func (this *SubscribeMessage) Topics() [][]byte {
	return this.topics
}

// AddTopic adds a single topic to the message, along with the corresponding QoS.
// An error is returned if QoS is invalid.
func (this *SubscribeMessage) AddTopic(topic []byte, qos byte) error {
	if !ValidQos(qos) {
		return fmt.Errorf("Invalid QoS %d", qos)
	}

	var i int
	var t []byte
	var found bool

	for i, t = range this.topics {
		if bytes.Equal(t, topic) {
			found = true
			break
		}
	}

	if found {
		this.qos[i] = qos
		return nil
	}

	this.topics = append(this.topics, topic)
	this.qos = append(this.qos, qos)
	this.dirty = true

	return nil
}

// RemoveTopic removes a single topic from the list of existing ones in the message.
// If topic does not exist it just does nothing.
func (this *SubscribeMessage) RemoveTopic(topic []byte) {
	var i int
	var t []byte
	var found bool

	for i, t = range this.topics {
		if bytes.Equal(t, topic) {
			found = true
			break
		}
	}

	if found {
		this.topics = append(this.topics[:i], this.topics[i+1:]...)
		this.qos = append(this.qos[:i], this.qos[i+1:]...)
	}

	this.dirty = true
}

// TopicExists checks to see if a topic exists in the list.
func (this *SubscribeMessage) TopicExists(topic []byte) bool {
	for _, t := range this.topics {
		if bytes.Equal(t, topic) {
			return true
		}
	}

	return false
}

// TopicQos returns the QoS level of a topic. If topic does not exist, QosFailure
// is returned.
func (this *SubscribeMessage) TopicQos(topic []byte) byte {
	for i, t := range this.topics {
		if bytes.Equal(t, topic) {
			return this.qos[i]
		}
	}

	return QosFailure
}

// Qos returns the list of QoS current in the message.
func (this *SubscribeMessage) Qos() []byte {
	return this.qos
}

func (this *SubscribeMessage) Len() int {
	if !this.dirty {
		return len(this.dbuf)
	}

	ml := this.msglen()

	if err := this.SetRemainingLength(int32(ml)); err != nil {
		return 0
	}

	return this.header.msglen() + ml
}

func (this *SubscribeMessage) Decode(src []byte) (int, error) {
	total := 0

	hn, err := this.header.decode(src[total:])
	total += hn
	if err != nil {
		return total, err
	}

	//this.packetId = binary.BigEndian.Uint16(src[total:])
	this.packetId = src[total : total+2]
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
	if total < len(src) && src[total] == DefiningIdentifiers {
		total++
		this.subscriptionIdentifier, n, err = lbDecode(src[total:])
		total += n
		if err != nil {
			return total, err
		}
		if this.subscriptionIdentifier == 0 || src[total] == DefiningIdentifiers {
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

	remlen := int(this.remlen) - (total - hn)
	var t []byte
	for remlen > 0 {
		t, n, err = readLPBytes(src[total:])
		total += n
		if err != nil {
			return total, err
		}

		this.topics = append(this.topics, t)

		this.qos = append(this.qos, src[total])
		total++

		remlen = remlen - n - 1
	}

	if len(this.topics) == 0 {
		return 0, ProtocolError //fmt.Errorf("subscribe/Decode: Empty topic list")
	}

	this.dirty = false

	return total, nil
}

func (this *SubscribeMessage) Encode(dst []byte) (int, error) {
	if !this.dirty {
		if len(dst) < len(this.dbuf) {
			return 0, fmt.Errorf("subscribe/Encode: Insufficient buffer size. Expecting %d, got %d.", len(this.dbuf), len(dst))
		}

		return copy(dst, this.dbuf), nil
	}

	hl := this.header.msglen()
	ml := this.msglen()

	if len(dst) < hl+ml {
		return 0, fmt.Errorf("subscribe/Encode: Insufficient buffer size. Expecting %d, got %d.", hl+ml, len(dst))
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

	if this.PacketId() == 0 {
		this.SetPacketId(uint16(atomic.AddUint64(&gPacketId, 1) & 0xffff))
		//this.packetId = uint16(atomic.AddUint64(&gPacketId, 1) & 0xffff)
	}

	n = copy(dst[total:], this.packetId)
	//binary.BigEndian.PutUint16(dst[total:], this.packetId)
	total += n

	tb := lbEncode(this.propertiesLen)
	copy(dst[total:], tb)
	total += len(tb)

	if this.subscriptionIdentifier > 0 && this.subscriptionIdentifier <= 268435455 {
		dst[total] = DefiningIdentifiers
		total++
		tb = lbEncode(this.subscriptionIdentifier)
		copy(dst[total:], tb)
		total += len(tb)
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

	for i, t := range this.topics {
		n, err = writeLPBytes(dst[total:], t)
		total += n
		if err != nil {
			return total, err
		}

		dst[total] = this.qos[i]
		total++
	}

	return total, nil
}

func (this *SubscribeMessage) msglen() int {
	// packet ID
	total := 2

	total += len(lbEncode(this.propertiesLen))
	if this.subscriptionIdentifier > 0 && this.subscriptionIdentifier <= 268435455 {
		total++
		total += len(lbEncode(this.subscriptionIdentifier))
	}
	for i := 0; i < len(this.userProperty); i++ {
		total++
		total += 2
		total += len(this.userProperty[i])
	}

	for _, t := range this.topics {
		total += 2 + len(t) + 1
	}
	return total
}
