package messagev5

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
	qos    []byte // 订阅选项的第0和1比特代表最大服务质量字段

	/**
	非本地（NoLocal）和发布保留（Retain As Published）订阅选项在客户端把消息发送给其他服务端的情况下，可以被用来实现桥接。

	已存在订阅的情况下不发送保留消息是很有用的，比如重连完成时客户端不确定订阅是否在之前的会话连接中被创建

	不发送保存的保留消息给新创建的订阅是很有用的，比如客户端希望接收变更通知且不需要知道最初的状态

	对于某个指示其不支持保留消息的服务端，发布保留和保留处理选项的所有有效值都将得到同样的结果：
			订阅时不发送任何保留消息，且所有消息的保留标志都会被设置为0
	**/

	// 订阅选项的第2比特表示非本地（No Local）选项。
	//	值为1，表示应用消息不能被转发给发布此消息的客户端。
	// 共享订阅时把非本地选项设为1将造成协议错误
	noLocal []byte
	// 订阅选项的第3比特表示发布保留（Retain As Published）选项。
	//   值为1，表示向此订阅转发应用消息时保持消息被发布时设置的保留（RETAIN）标志。
	//   值为0，表示向此订阅转发应用消息时把保留标志设置为0。当订阅建立之后，发送保留消息时保留标志设置为1。
	retainAsPub []byte
	// 订阅选项的第4和5比特表示保留操作（Retain Handling）选项。
	//	此选项指示当订阅建立时，是否发送保留消息。
	//	此选项不影响之后的任何保留消息的发送。
	//	如果没有匹配主题过滤器的保留消息，则此选项所有值的行为都一样。值可以设置为：
	//		0 = 订阅建立时发送保留消息
	//		1 = 订阅建立时，若该订阅当前不存在则发送保留消息
	//		2 = 订阅建立时不要发送保留消息
	//	保留操作的值设置为3将造成协议错误（Protocol Error）。
	retainHandling []byte
	// 订阅选项的第6和7比特为将来所保留。服务端必须把此保留位非0的SUBSCRIBE报文当做无效报文
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
		msgstr = fmt.Sprintf("%s, Topic[%d]=%q，Qos：%d，noLocal：%d，Retain As Publish：%d，RetainHandling：%d",
			msgstr, i, string(t), this.qos[i], this.noLocal[i], this.retainAsPub[i], this.retainHandling[i])
	}

	return msgstr
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
	this.noLocal = append(this.noLocal, 0)
	this.retainAsPub = append(this.retainAsPub, 0)
	this.retainHandling = append(this.retainHandling, 0)
	this.dirty = true

	return nil
}
func (this *SubscribeMessage) AddTopicAll(topic []byte, qos byte, noLocal, retainAsPub bool, retainHandling byte) error {
	if !ValidQos(qos) {
		return fmt.Errorf("Invalid QoS %d", qos)
	}
	if retainHandling > 2 {
		return fmt.Errorf("Invalid Sub Option RetainHandling %d", retainHandling)
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
	if noLocal {
		this.noLocal = append(this.noLocal, 1)
	} else {
		this.noLocal = append(this.noLocal, 0)
	}
	if retainAsPub {
		this.retainAsPub = append(this.retainAsPub, 1)
	} else {
		this.retainAsPub = append(this.retainAsPub, 0)
	}

	this.retainHandling = append(this.retainHandling, retainHandling)
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
		this.noLocal = append(this.noLocal[:i], this.noLocal[i+1:]...)
		this.retainAsPub = append(this.retainAsPub[:i], this.retainAsPub[i+1:]...)
		this.retainHandling = append(this.retainHandling[:i], this.retainHandling[i+1:]...)
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
func (this *SubscribeMessage) TopicNoLocal(topic []byte) bool {
	for i, t := range this.topics {
		if bytes.Equal(t, topic) {
			return this.noLocal[i] > 0
		}
	}
	return false
}
func (this *SubscribeMessage) SetTopicNoLocal(topic []byte, noLocal bool) {
	for i, t := range this.topics {
		if bytes.Equal(t, topic) {
			if noLocal {
				this.noLocal[i] = 1
			} else {
				this.noLocal[i] = 0
			}
			this.dirty = true
			return
		}
	}
}
func (this *SubscribeMessage) TopicRetainAsPublished(topic []byte) bool {
	for i, t := range this.topics {
		if bytes.Equal(t, topic) {
			return this.retainAsPub[i] > 0
		}
	}
	return false
}
func (this *SubscribeMessage) SetTopicRetainAsPublished(topic []byte, rap bool) {
	for i, t := range this.topics {
		if bytes.Equal(t, topic) {
			if rap {
				this.retainAsPub[i] = 1
			} else {
				this.retainAsPub[i] = 0
			}
			this.dirty = true
			return
		}
	}
}
func (this *SubscribeMessage) TopicRetainHandling(topic []byte) RetainHandling {
	for i, t := range this.topics {
		if bytes.Equal(t, topic) {
			return RetainHandling(this.retainHandling[i])
		}
	}
	return CanSendRetain
}
func (this *SubscribeMessage) SetTopicRetainHandling(topic []byte, hand RetainHandling) {
	for i, t := range this.topics {
		if bytes.Equal(t, topic) {
			this.retainHandling[i] = byte(hand)
			this.dirty = true
			return
		}
	}
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

		this.qos = append(this.qos, src[total]&3)
		if src[total]&3 == 3 {
			return 0, ProtocolError
		}
		this.noLocal = append(this.noLocal, (src[total]&4)>>2)
		this.retainAsPub = append(this.retainAsPub, (src[total]&8)>>3)
		this.retainHandling = append(this.retainHandling, (src[total]&48)>>4)
		if (src[total]&48)>>4 == 3 {
			return 0, ProtocolError
		}
		if src[total]>>6 != 0 {
			return 0, ProtocolError
		}
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

	ml := this.msglen()
	hl := this.header.msglen()

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
		// 订阅选项
		subOp := byte(0)
		subOp |= this.qos[i]
		subOp |= this.noLocal[i] << 2
		subOp |= this.retainAsPub[i] << 3
		subOp |= this.retainHandling[i] << 4
		dst[total] = subOp
		total++
	}

	return total, nil
}
func (this *SubscribeMessage) build() {
	// packet ID
	total := 2

	if this.subscriptionIdentifier > 0 && this.subscriptionIdentifier <= 268435455 {
		total++
		total += len(lbEncode(this.subscriptionIdentifier))
	}
	for i := 0; i < len(this.userProperty); i++ {
		total++
		total += 2
		total += len(this.userProperty[i])
	}
	this.propertiesLen = uint32(total - 2)
	total += len(lbEncode(this.propertiesLen))
	for _, t := range this.topics {
		total += 2 + len(t) + 1
	}
	_ = this.SetRemainingLength(int32(total))
}
func (this *SubscribeMessage) msglen() int {
	this.build()
	return int(this.remlen)
}
