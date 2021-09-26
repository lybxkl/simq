package messagev5

import (
	"bytes"
	"fmt"
	"sync/atomic"
)

// An UNSUBSCRIBE Packet is sent by the Client to the Server, to unsubscribe from topics.
type UnsubscribeMessage struct {
	header
	// === 可变报头 ===
	//  属性长度
	propertyLen  uint32
	userProperty [][]byte
	// 载荷
	topics [][]byte
}

func (this *UnsubscribeMessage) PropertyLen() uint32 {
	return this.propertyLen
}

func (this *UnsubscribeMessage) SetPropertyLen(propertyLen uint32) {
	this.propertyLen = propertyLen
	this.dirty = true
}

func (this *UnsubscribeMessage) UserProperty() [][]byte {
	return this.userProperty
}

func (this *UnsubscribeMessage) AddUserPropertys(userProperty [][]byte) {
	this.userProperty = append(this.userProperty, userProperty...)
	this.dirty = true
}
func (this *UnsubscribeMessage) AddUserProperty(userProperty []byte) {
	this.userProperty = append(this.userProperty, userProperty)
	this.dirty = true
}

var _ Message = (*UnsubscribeMessage)(nil)

// NewUnsubscribeMessage creates a new UNSUBSCRIBE message.
func NewUnsubscribeMessage() *UnsubscribeMessage {
	msg := &UnsubscribeMessage{}
	msg.SetType(UNSUBSCRIBE)

	return msg
}

func (this UnsubscribeMessage) String() string {
	msgstr := fmt.Sprintf("%s", this.header)
	msgstr = fmt.Sprintf("%s, PropertiesLen=%v,  User Properties=%v", msgstr, this.propertyLen, this.UserProperty())

	for i, t := range this.topics {
		msgstr = fmt.Sprintf("%s, Topic%d=%s", msgstr, i, string(t))
	}

	return msgstr
}

// Topics returns a list of topics sent by the Client.
func (this *UnsubscribeMessage) Topics() [][]byte {
	return this.topics
}

// AddTopic adds a single topic to the message.
func (this *UnsubscribeMessage) AddTopic(topic []byte) {
	if this.TopicExists(topic) {
		return
	}

	this.topics = append(this.topics, topic)
	this.dirty = true
}

// RemoveTopic removes a single topic from the list of existing ones in the message.
// If topic does not exist it just does nothing.
func (this *UnsubscribeMessage) RemoveTopic(topic []byte) {
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
	}

	this.dirty = true
}

// TopicExists checks to see if a topic exists in the list.
func (this *UnsubscribeMessage) TopicExists(topic []byte) bool {
	for _, t := range this.topics {
		if bytes.Equal(t, topic) {
			return true
		}
	}

	return false
}

func (this *UnsubscribeMessage) Len() int {
	if !this.dirty {
		return len(this.dbuf)
	}

	ml := this.msglen()

	if err := this.SetRemainingLength(int32(ml)); err != nil {
		return 0
	}

	return this.header.msglen() + ml
}

// Decode reads from the io.Reader parameter until a full message is decoded, or
// when io.Reader returns EOF or error. The first return value is the number of
// bytes read from io.Reader. The second is error if Decode encounters any problems.
func (this *UnsubscribeMessage) Decode(src []byte) (int, error) {
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
	if total < len(src) && len(src[total:]) >= 4 {
		this.propertyLen, n, err = lbDecode(src[total:])
		total += n
		if err != nil {
			return total, err
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
	remlen := int(this.remlen) - (total - hn)
	var t []byte
	for remlen > 0 {
		t, n, err = readLPBytes(src[total:])
		total += n
		if err != nil {
			return total, err
		}

		this.topics = append(this.topics, t)
		remlen = remlen - n - 1
	}

	if len(this.topics) == 0 {
		return 0, ProtocolError // fmt.Errorf("unsubscribe/Decode: Empty topic list")
	}

	this.dirty = false

	return total, nil
}

// Encode returns an io.Reader in which the encoded bytes can be read. The second
// return value is the number of bytes encoded, so the caller knows how many bytes
// there will be. If Encode returns an error, then the first two return values
// should be considered invalid.
// Any changes to the message after Encode() is called will invalidate the io.Reader.
func (this *UnsubscribeMessage) Encode(dst []byte) (int, error) {
	if !this.dirty {
		if len(dst) < len(this.dbuf) {
			return 0, fmt.Errorf("unsubscribe/Encode: Insufficient buffer size. Expecting %d, got %d.", len(this.dbuf), len(dst))
		}

		return copy(dst, this.dbuf), nil
	}

	ml := this.msglen()
	hl := this.header.msglen()

	if len(dst) < hl+ml {
		return 0, fmt.Errorf("unsubscribe/Encode: Insufficient buffer size. Expecting %d, got %d.", hl+ml, len(dst))
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

	b := lbEncode(this.propertyLen)
	copy(dst[total:], b)
	total += len(b)
	for i := 0; i < len(this.userProperty); i++ {
		dst[total] = UserProperty
		total++
		n, err = writeLPBytes(dst[total:], this.userProperty[i])
		total += n
		if err != nil {
			return total, err
		}
	}
	for _, t := range this.topics {
		n, err = writeLPBytes(dst[total:], t)
		total += n
		if err != nil {
			return total, err
		}
	}

	return total, nil
}
func (this *UnsubscribeMessage) build() {
	total := 0
	for _, t := range this.userProperty {
		total += 1 + 2 + len(t)
	}
	this.propertyLen = uint32(total)

	total += 2 // packet ID
	for _, t := range this.topics {
		total += 2 + len(t)
	}
	total += len(lbEncode(this.propertyLen))
	_ = this.SetRemainingLength(int32(total))
}
func (this *UnsubscribeMessage) msglen() int {
	this.build()
	return int(this.remlen)
}
