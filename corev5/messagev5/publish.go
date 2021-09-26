package messagev5

import (
	"encoding/binary"
	"fmt"
	"sync/atomic"
)

// PublishMessage A PUBLISH Control Packet is sent from a Client to a Server or from Server to a Client
// to transport an Application Message.
type PublishMessage struct {
	header

	// === 可变报头 ===
	topic []byte // 主题
	// qos > 0 需要要报文标识符，数据放在header里面
	// 位置还是在topic后面

	// 属性
	propertiesLen          uint32   // 属性长度，变长字节整数
	payloadFormatIndicator byte     // 载荷格式指示，默认0 ， 服务端必须把接收到的应用消息中的载荷格式指示原封不动的发给所有的订阅者
	messageExpiryInterval  uint32   // 消息过期间隔，没有过期间隔，则应用消息不会过期 如果消息过期间隔已过期，服务端还没开始向匹配的订阅者交付该消息，则服务端必须删除该订阅者的消息副本
	topicAlias             uint16   // 主题别名，可以没有传，传了就不能为0，并且不能发超过connack中主题别名最大值
	responseTopic          []byte   // 响应主题
	correlationData        []byte   // 对比数据
	userProperty           [][]byte // 用户属性 , 保证顺序
	subscriptionIdentifier uint32   // 一个变长字节整数表示的订阅标识符 从1到268,435,455。订阅标识符的值为0将造成协议错误
	contentType            []byte   // 内容类型 UTF-8编码

	// === 载荷 ===
	payload []byte // 用固定报头中的剩余长度字段的值减去可变报头的长度。包含零长度有效载荷的PUBLISH报文是合法的
}

var _ Message = (*PublishMessage)(nil)

// NewPublishMessage creates a new PUBLISH message.
func NewPublishMessage() *PublishMessage {
	msg := &PublishMessage{}
	msg.SetType(PUBLISH)

	return msg
}
func (m *PublishMessage) Copy() *PublishMessage {
	p := NewPublishMessage()
	cp := make([]byte, p.Len())
	_, _ = m.Encode(cp)
	_, _ = p.Decode(cp)
	return p
}
func (this PublishMessage) String() string {
	return fmt.Sprintf("%s, Topic=%q, QoS=%d, Retained=%t, Dup=%t, Payload=%s, "+
		"PropertiesLen=%d, Payload Format Indicator=%d, Message Expiry Interval=%d, Topic Alias=%v, Response Topic=%s, "+
		"Correlation Data=%s, User Property=%s, Subscription Identifier=%v, Content Type=%s",
		this.header, this.topic, this.QoS(), this.Retain(), this.Dup(), this.payload,
		this.propertiesLen, this.payloadFormatIndicator, this.messageExpiryInterval, this.topicAlias, this.responseTopic,
		this.correlationData, this.userProperty, this.subscriptionIdentifier, this.contentType)
}
func (this *PublishMessage) PropertiesLen() uint32 {
	return this.propertiesLen
}

func (this *PublishMessage) SetPropertiesLen(propertiesLen uint32) {
	this.propertiesLen = propertiesLen
	this.dirty = true
}

func (this *PublishMessage) PayloadFormatIndicator() byte {
	return this.payloadFormatIndicator
}

func (this *PublishMessage) SetPayloadFormatIndicator(payloadFormatIndicator byte) {
	this.payloadFormatIndicator = payloadFormatIndicator
	this.dirty = true
}

func (this *PublishMessage) MessageExpiryInterval() uint32 {
	return this.messageExpiryInterval
}

func (this *PublishMessage) SetMessageExpiryInterval(messageExpiryInterval uint32) {
	this.messageExpiryInterval = messageExpiryInterval
	this.dirty = true
}

func (this *PublishMessage) TopicAlias() uint16 {
	return this.topicAlias
}

func (this *PublishMessage) SetTopicAlias(topicAlias uint16) {
	this.topicAlias = topicAlias
	this.dirty = true
}
func (this *PublishMessage) SetNilTopicAndAlias(alias uint16) {
	this.topicAlias = alias
	this.topic = nil
	this.dirty = true
}
func (this *PublishMessage) ResponseTopic() []byte {
	return this.responseTopic
}

func (this *PublishMessage) SetResponseTopic(responseTopic []byte) {
	this.responseTopic = responseTopic
	this.dirty = true
}

func (this *PublishMessage) CorrelationData() []byte {
	return this.correlationData
}

func (this *PublishMessage) SetCorrelationData(correlationData []byte) {
	this.correlationData = correlationData
	this.dirty = true
}

func (this *PublishMessage) UserProperty() [][]byte {
	return this.userProperty
}

func (this *PublishMessage) AddUserPropertys(userProperty [][]byte) {
	this.userProperty = append(this.userProperty, userProperty...)
	this.dirty = true
}
func (this *PublishMessage) AddUserProperty(userProperty []byte) {
	this.userProperty = append(this.userProperty, userProperty)
	this.dirty = true
}

func (this *PublishMessage) SubscriptionIdentifier() uint32 {
	return this.subscriptionIdentifier
}

func (this *PublishMessage) SetSubscriptionIdentifier(subscriptionIdentifier uint32) {
	this.subscriptionIdentifier = subscriptionIdentifier
	this.dirty = true
}

func (this *PublishMessage) ContentType() []byte {
	return this.contentType
}

func (this *PublishMessage) SetContentType(contentType []byte) {
	this.contentType = contentType
	this.dirty = true
}

// Dup returns the value specifying the duplicate delivery of a PUBLISH Control Packet.
// If the DUP flag is set to 0, it indicates that this is the first occasion that the
// Client or Server has attempted to send this MQTT PUBLISH Packet. If the DUP flag is
// set to 1, it indicates that this might be re-delivery of an earlier attempt to send
// the Packet.
func (this *PublishMessage) Dup() bool {
	return ((this.Flags() >> 3) & 0x1) == 1
}

// SetDup sets the value specifying the duplicate delivery of a PUBLISH Control Packet.
func (this *PublishMessage) SetDup(v bool) {
	if v {
		this.mtypeflags[0] |= 0x8 // 00001000
	} else {
		this.mtypeflags[0] &= 247 // 11110111
	}
}

// Retain returns the value of the RETAIN flag. This flag is only used on the PUBLISH
// Packet. If the RETAIN flag is set to 1, in a PUBLISH Packet sent by a Client to a
// Server, the Server MUST store the Application Message and its QoS, so that it can be
// delivered to future subscribers whose subscriptions match its topic name.
func (this *PublishMessage) Retain() bool {
	return (this.Flags() & 0x1) == 1
}

// SetRetain sets the value of the RETAIN flag.
func (this *PublishMessage) SetRetain(v bool) {
	if v {
		this.mtypeflags[0] |= 0x1 // 00000001
	} else {
		this.mtypeflags[0] &= 254 // 11111110
	}
}

// QoS returns the field that indicates the level of assurance for delivery of an
// Application Message. The values are QosAtMostOnce, QosAtLeastOnce and QosExactlyOnce.
func (this *PublishMessage) QoS() byte {
	return (this.Flags() >> 1) & 0x3
}

// SetQoS sets the field that indicates the level of assurance for delivery of an
// Application Message. The values are QosAtMostOnce, QosAtLeastOnce and QosExactlyOnce.
// An error is returned if the value is not one of these.
func (this *PublishMessage) SetQoS(v byte) error {
	if v != 0x0 && v != 0x1 && v != 0x2 {
		return fmt.Errorf("publish/SetQoS: Invalid QoS %d.", v)
	}

	this.mtypeflags[0] = (this.mtypeflags[0] & 249) | (v << 1) // 249 = 11111001
	this.dirty = true
	return nil
}

// Topic returns the the topic name that identifies the information channel to which
// payload data is published.
func (this *PublishMessage) Topic() []byte {
	return this.topic
}

// SetTopic sets the the topic name that identifies the information channel to which
// payload data is published. An error is returned if ValidTopic() is falbase.
func (this *PublishMessage) SetTopic(v []byte) error {
	if !ValidTopic(v) {
		return fmt.Errorf("publish/SetTopic: Invalid topic name (%s). Must not be empty or contain wildcard characters", string(v))
	}

	this.topic = v
	this.dirty = true

	return nil
}

// Payload returns the application message that's part of the PUBLISH message.
func (this *PublishMessage) Payload() []byte {
	return this.payload
}

// SetPayload sets the application message that's part of the PUBLISH message.
func (this *PublishMessage) SetPayload(v []byte) {
	this.payload = v
	this.dirty = true
}

func (this *PublishMessage) Len() int {
	if !this.dirty {
		return len(this.dbuf)
	}

	ml := this.msglen()

	if err := this.SetRemainingLength(int32(ml)); err != nil {
		return 0
	}

	return this.header.msglen() + ml
}

func (this *PublishMessage) Decode(src []byte) (int, error) {
	total := 0

	hn, err := this.header.decode(src[total:])
	total += hn
	if err != nil {
		return total, err
	}

	n := 0
	// === 可变报头 ===
	this.topic, n, err = readLPBytes(src[total:])
	total += n
	if err != nil {
		return total, err
	}

	// The packet identifier field is only present in the PUBLISH packets where the
	// QoS level is 1 or 2
	if this.QoS() != 0 {
		//this.packetId = binary.BigEndian.Uint16(src[total:])

		this.packetId = CopyLen(src[total:total+2], 2) //src[total : total+2]
		total += 2
	}

	propertiesLen, n, err := lbDecode(src[total:]) // 属性长度
	if err != nil {
		return 0, err
	}
	total += n
	this.propertiesLen = propertiesLen

	if total < len(src) && src[total] == LoadFormatDescription { // 载荷格式
		total++
		this.payloadFormatIndicator = src[total]
		total++
		if total < len(src) && src[total] == LoadFormatDescription {
			return 0, ProtocolError
		}
	}
	if total < len(src) && src[total] == MessageExpirationTime { // 消息过期间隔
		total++
		this.messageExpiryInterval = binary.BigEndian.Uint32(src[total:])
		total += 4
		if total < len(src) && src[total] == MessageExpirationTime {
			return 0, ProtocolError
		}
	}
	if total < len(src) && src[total] == ThemeAlias { // 主题别名
		total++
		this.topicAlias = binary.BigEndian.Uint16(src[total:])
		total += 2
		if this.topicAlias == 0 || (total < len(src) && src[total] == ThemeAlias) {
			return 0, ProtocolError
		}
	}
	if this.topicAlias == 0 { // 没有主题别名才验证topic
		if !ValidTopic(this.topic) {
			return total, fmt.Errorf("publish/Decode: Invalid topic name (%s). Must not be empty or contain wildcard characters", string(this.topic))
		}
	}
	if total < len(src) && src[total] == ResponseTopic { // 响应主题
		total++
		this.responseTopic, n, err = readLPBytes(src[total:])
		total += n
		if err != nil {
			return total, err
		}
		if len(this.responseTopic) == 0 || (total < len(src) && src[total] == ResponseTopic) {
			return 0, ProtocolError
		}
	}
	if total < len(src) && src[total] == RelatedData { // 对比数据
		total++
		this.correlationData, n, err = readLPBytes(src[total:])
		total += n
		if err != nil {
			return total, err
		}
		if len(this.correlationData) == 0 || (total < len(src) && src[total] == RelatedData) {
			return 0, ProtocolError
		}
	}
	if total < len(src) && src[total] == UserProperty { // 用户属性
		total++
		this.userProperty = make([][]byte, 0)
		userP1, n1, err1 := readLPBytes(src[total:])
		total += n1
		if err1 != nil {
			return total, err1
		}
		this.userProperty = append(this.userProperty, userP1)
		for total < len(src) && src[total] == UserProperty {
			total++
			userP1, n1, err1 = readLPBytes(src[total:])
			total += n1
			if err1 != nil {
				return total, err1
			}
			this.userProperty = append(this.userProperty, userP1)
		}
	}
	if total < len(src) && src[total] == DefiningIdentifiers { // 订阅标识符
		total++
		this.subscriptionIdentifier, n, err = lbDecode(src[total:])
		if err != nil {
			return 0, ProtocolError
		}
		total += n
		if this.subscriptionIdentifier == 0 || (total < len(src) && src[total] == DefiningIdentifiers) {
			return 0, ProtocolError
		}
	}
	if total < len(src) && src[total] == ContentType { // 内容类型
		total++
		this.contentType, n, err = readLPBytes(src[total:])
		if err != nil {
			return 0, ProtocolError
		}
		total += n
		if len(this.contentType) == 0 || (total < len(src) && src[total] == ContentType) {
			return 0, ProtocolError
		}
	}
	// === 载荷 ===
	l := int(this.remlen) - (total - hn)
	this.payload = CopyLen(src[total:total+l], l)
	total += len(this.payload)

	this.dirty = false

	return total, nil
}

func (this *PublishMessage) Encode(dst []byte) (int, error) {
	if !this.dirty {
		if len(dst) < len(this.dbuf) {
			return 0, fmt.Errorf("publish/Encode: Insufficient buffer size. Expecting %d, got %d.", len(this.dbuf), len(dst))
		}

		return copy(dst, this.dbuf), nil
	}
	if len(this.topic) == 0 && this.topicAlias == 0 {
		return 0, fmt.Errorf("publish/Encode: Topic name is empty, and topic alice <= 0.")
	}

	if len(this.payload) == 0 {
		return 0, fmt.Errorf("publish/Encode: Payload is empty.")
	}

	ml := this.msglen()
	hl := this.header.msglen()

	if err := this.SetRemainingLength(int32(ml)); err != nil {
		return 0, err
	}

	if len(dst) < hl+ml {
		return 0, fmt.Errorf("publish/Encode: Insufficient buffer size. Expecting %d, got %d.", hl+ml, len(dst))
	}

	total := 0

	n, err := this.header.encode(dst[total:])
	total += n
	if err != nil {
		return total, err
	}

	n, err = writeLPBytes(dst[total:], this.topic)
	total += n
	if err != nil {
		return total, err
	}

	// The packet identifier field is only present in the PUBLISH packets where the QoS level is 1 or 2
	if this.QoS() != 0 {
		if this.PacketId() == 0 {
			this.SetPacketId(uint16(atomic.AddUint64(&gPacketId, 1) & 0xffff))
			//this.packetId = uint16(atomic.AddUint64(&gPacketId, 1) & 0xffff)
		}

		n = copy(dst[total:], this.packetId)
		//binary.BigEndian.PutUint16(dst[total:], this.packetId)
		total += n
	}
	// === 属性 ===
	b := lbEncode(this.propertiesLen)
	copy(dst[total:], b)
	total += len(b)
	if this.payloadFormatIndicator != 0 { // 载荷格式指示
		dst[total] = LoadFormatDescription
		total++
		dst[total] = this.payloadFormatIndicator
		total++
	}
	if this.messageExpiryInterval > 0 { // 消息过期间隔
		dst[total] = MessageExpirationTime
		total++
		binary.BigEndian.PutUint32(dst[total:], this.messageExpiryInterval)
		total += 4
	}
	if this.topicAlias > 0 { // 主题别名
		dst[total] = ThemeAlias
		total++
		binary.BigEndian.PutUint16(dst[total:], this.topicAlias)
		total += 2
	}
	if len(this.responseTopic) > 0 { // 响应主题
		dst[total] = ResponseTopic
		total++
		n, err = writeLPBytes(dst[total:], this.responseTopic)
		total += n
		if err != nil {
			return total, err
		}
	}
	if len(this.correlationData) > 0 { // 对比数据
		dst[total] = RelatedData
		total++
		n, err = writeLPBytes(dst[total:], this.correlationData)
		total += n
		if err != nil {
			return total, err
		}
	}
	if len(this.userProperty) > 0 { // 用户属性
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
	if this.subscriptionIdentifier > 0 && this.subscriptionIdentifier < 168435455 { // 订阅标识符
		dst[total] = DefiningIdentifiers
		total++
		b1 := lbEncode(this.subscriptionIdentifier)
		copy(dst[total:], b1)
		total += len(b1)
	}
	if len(this.contentType) > 0 { // 内容类型
		dst[total] = ContentType
		total++
		n, err = writeLPBytes(dst[total:], this.contentType)
		total += n
		if err != nil {
			return total, err
		}
	}

	copy(dst[total:], this.payload)
	total += len(this.payload)

	return total, nil
}
func (this *PublishMessage) build() {
	total := 0

	total += 2 // 主题
	total += len(this.topic)
	if this.QoS() != 0 {
		total += 2
	}
	tag := total
	if this.payloadFormatIndicator != 0 { // 载荷格式指示
		total += 2
	}
	if this.messageExpiryInterval > 0 { // 消息过期间隔
		total += 5
	}
	if this.topicAlias > 0 { // 主题别名
		total += 3
	}
	if len(this.responseTopic) > 0 { // 响应主题
		total++
		total += 2
		total += len(this.responseTopic)
	}
	if len(this.correlationData) > 0 { // 对比数据
		total++
		total += 2
		total += len(this.correlationData)
	}
	if len(this.userProperty) > 0 { // 用户属性
		for i := 0; i < len(this.userProperty); i++ {
			total++
			total += 2
			total += len(this.userProperty[i])
		}
	}
	if this.subscriptionIdentifier > 0 && this.subscriptionIdentifier < 168435455 { // 订阅标识符
		total++
		b1 := lbEncode(this.subscriptionIdentifier)
		total += len(b1)
	}
	if len(this.contentType) > 0 { // 内容类型
		total++
		total += 2
		total += len(this.contentType)
	}
	propertiesLen := uint32(total - tag) // 可变的属性长度
	this.propertiesLen = propertiesLen
	total += len(lbEncode(propertiesLen))
	total += len(this.payload)
	_ = this.SetRemainingLength(int32(total))
}
func (this *PublishMessage) msglen() int {
	this.build()
	return int(this.remlen)
}
