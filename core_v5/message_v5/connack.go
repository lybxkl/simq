package message

import (
	"encoding/binary"
	"fmt"
)

// ConnackMessage The CONNACK Packet is the packet sent by the Server in response to a CONNECT Packet
// received from a Client. The first packet sent from the Server to the Client MUST
// be a CONNACK Packet [MQTT-3.2.0-1].
//
// If the Client does not receive a CONNACK Packet from the Server within a reasonable
// amount of time, the Client SHOULD close the Network Connection. A "reasonable" amount
// of time depends on the type of application and the communications infrastructure.
type ConnackMessage struct {
	buildTag bool
	header

	sessionPresent bool
	reasonCode     ReasonCode

	// Connack 属性
	propertiesLen                   int               // 属性长度
	sessionExpiryInterval           uint32            // 会话过期间隔
	receiveMaximum                  uint16            // 接收最大值
	maxQos                          byte              // 最大服务质量
	retainAvailable                 byte              // 保留可用
	maxPacketSize                   uint32            // 最大报文长度是 MQTT 控制报文的总长度
	assignedIdentifier              string            // 分配的客户标识符，分配是因为connect报文中，clientid长度为0，由服务器生成
	topicAliasMax                   uint16            // 主题别名最大值（Topic Alias Maximum）
	reasonStr                       string            // 原因字符串 不应该被客户端所解析，如果加上原因字符串之后的CONNACK报文长度超出了客户端指定的最大报文长度，则服务端不能发送此原因字符串
	userProperties                  map[string]string // 用户属性 如果加上用户属性之后的CONNACK报文长度超出了客户端指定的最大报文长度，则服务端不能发送此属性
	wildcardSubscriptionAvailable   byte              // 通配符可用
	subscriptionIdentifierAvailable byte              // 订阅标识符可用
	sharedSubscriptionAvailable     byte              // 支持共享订阅
	serverKeepAlive                 uint16            // 服务保持连接时间 如果服务端发送了服务端保持连接（Server Keep Alive）属性，客户端必须使用此值代替其在CONNECT报文中发送的保持连接时间值
	responseInformation             string            // 响应信息 以UTF-8编码的字符串，作为创建响应主题（Response Topic）的基本信息
	serverReference                 string            // 服务端参考
	authMethod                      string            // 认证方法，必须与connect中的一致
	authData                        []byte            // 认证数据 此数据的内容由认证方法和已交换的认证数据状态定义
	// 无载荷
}

var _ Message = (*ConnackMessage)(nil)

// NewConnackMessage creates a new CONNACK message
func NewConnackMessage() *ConnackMessage {
	msg := &ConnackMessage{}
	msg.SetType(CONNACK)

	return msg
}

// String returns a string representation of the CONNACK message
func (this ConnackMessage) String() string {
	return fmt.Sprintf("Header==>> \n\t\t%s\nVariable header==>> \n\t\tSession Present=%t\n\t\tReason code=%s\n\t\t"+
		"Properties\n\t\t\t"+
		"Length=%v\n\t\t\tSession Expiry Interval=%v\n\t\t\tReceive Maximum=%v\n\t\t\tMaximum QoS=%v\n\t\t\t"+
		"Retain Available=%b\n\t\t\tMax Packet Size=%v\n\t\t\tAssignedIdentifier=%v\n\t\t\t"+
		"Topic Alias Max=%v\n\t\t\tReason Str=%v\n\t\t\tUser Properties=%v\n\t\t\t"+
		"Wildcard Subscription Available=%b\n\t\t\tSubscription Identifier Available=%b\n\t\t\tShared Subscription Available=%b\n\t\t\tServer Keep Alive=%v\n\t\t\t"+
		"Response Information=%v\n\t\t\tServer Reference=%v\n\t\t\tAuth Method=%v\n\t\t\tAuth Data=%v\n\t\t",
		this.header,

		this.sessionPresent, this.reasonCode,

		this.propertiesLen,
		this.sessionExpiryInterval,
		this.receiveMaximum,
		this.maxQos,
		this.retainAvailable,
		this.maxPacketSize,
		this.assignedIdentifier,
		this.topicAliasMax,
		this.reasonStr,
		this.userProperties,

		this.wildcardSubscriptionAvailable,
		this.subscriptionIdentifierAvailable,
		this.sharedSubscriptionAvailable,
		this.serverKeepAlive,
		this.responseInformation,
		this.serverReference,
		this.authMethod,
		this.authData,
	)
}
func (this *ConnackMessage) build() {
	if this.buildTag {
		return
	}
	propertiesLen := 2
	// 属性
	if this.sessionExpiryInterval > 0 { // 会话过期间隔
		propertiesLen += 5
	}
	if this.receiveMaximum > 0 && this.receiveMaximum < 65535 { // 接收最大值
		propertiesLen += 3
	} else {
		this.receiveMaximum = 65535
	}
	if this.maxQos > 0 { // 最大服务质量，正常都会编码
		propertiesLen += 2
	}
	if this.retainAvailable != 1 { // 保留可用
		propertiesLen += 2
	}
	if this.maxPacketSize != 1 { // 最大报文长度
		propertiesLen += 5
	}
	if len(this.assignedIdentifier) > 0 { // 分配客户标识符
		propertiesLen++
		propertiesLen += len(this.assignedIdentifier)
	}
	if this.topicAliasMax > 0 { // 主题别名最大值
		propertiesLen += 3
	}
	if len(this.reasonStr) > 0 { // 原因字符串
		propertiesLen++
		propertiesLen += len(this.reasonStr)
	}
	for k, v := range this.userProperties { // 用户属性
		propertiesLen++
		propertiesLen += len(k)
		propertiesLen++
		propertiesLen += len(v)
	}
	if this.wildcardSubscriptionAvailable != 1 { // 通配符订阅可用
		propertiesLen += 2
	}
	if this.subscriptionIdentifierAvailable != 1 { // 订阅标识符可用
		propertiesLen += 2
	}
	if this.sharedSubscriptionAvailable != 1 { // 共享订阅可用
		propertiesLen += 2
	}
	if this.serverKeepAlive > 0 { // 服务端保持连接
		propertiesLen += 3
	}
	if len(this.responseInformation) > 0 { // 响应信息
		propertiesLen++
		propertiesLen += len(this.responseInformation)
	}
	if len(this.serverReference) > 0 { // 服务端参考
		propertiesLen++
		propertiesLen += len(this.serverReference)
	}
	if len(this.authMethod) > 0 { // 认证方法
		propertiesLen++
		propertiesLen += len(this.authMethod)
	}
	if len(this.authData) > 0 { // 认证数据
		propertiesLen++
		propertiesLen += len(this.authData)
	}
	this.propertiesLen = propertiesLen
	if err := this.SetRemainingLength(int32(propertiesLen)); err != nil {
	}
}
func (this *ConnackMessage) PropertiesLen() int {
	return this.propertiesLen
}

func (this *ConnackMessage) SetPropertiesLen(propertiesLen int) {
	this.propertiesLen = propertiesLen
	this.dirty = true
}

func (this *ConnackMessage) SessionExpiryInterval() uint32 {
	return this.sessionExpiryInterval
}

func (this *ConnackMessage) SetSessionExpiryInterval(sessionExpiryInterval uint32) {
	this.sessionExpiryInterval = sessionExpiryInterval
	this.dirty = true
}

func (this *ConnackMessage) ReceiveMaximum() uint16 {
	return this.receiveMaximum
}

func (this *ConnackMessage) SetReceiveMaximum(receiveMaximum uint16) {
	this.receiveMaximum = receiveMaximum
	this.dirty = true
}

func (this *ConnackMessage) MaxQos() byte {
	return this.maxQos
}

func (this *ConnackMessage) SetMaxQos(maxQos byte) {
	this.maxQos = maxQos
	this.dirty = true
}

func (this *ConnackMessage) RetainAvailable() byte {
	return this.retainAvailable
}

func (this *ConnackMessage) SetRetainAvailable(retainAvailable byte) {
	this.retainAvailable = retainAvailable
	this.dirty = true
}

func (this *ConnackMessage) MaxPacketSize() uint32 {
	return this.maxPacketSize
}

func (this *ConnackMessage) SetMaxPacketSize(maxPacketSize uint32) {
	this.maxPacketSize = maxPacketSize
	this.dirty = true
}

func (this *ConnackMessage) AssignedIdentifier() string {
	return this.assignedIdentifier
}

func (this *ConnackMessage) SetAssignedIdentifier(assignedIdentifier string) {
	this.assignedIdentifier = assignedIdentifier
	this.dirty = true
}

func (this *ConnackMessage) TopicAliasMax() uint16 {
	return this.topicAliasMax
}

func (this *ConnackMessage) SetTopicAliasMax(topicAliasMax uint16) {
	this.topicAliasMax = topicAliasMax
	this.dirty = true
}

func (this *ConnackMessage) ReasonStr() string {
	return this.reasonStr
}

func (this *ConnackMessage) SetReasonStr(reasonStr string) {
	this.reasonStr = reasonStr
	this.dirty = true
}

func (this *ConnackMessage) UserProperties() map[string]string {
	return this.userProperties
}

func (this *ConnackMessage) SetUserProperties(userProperties map[string]string) {
	this.userProperties = userProperties
	this.dirty = true
}

func (this *ConnackMessage) WildcardSubscriptionAvailable() byte {
	return this.wildcardSubscriptionAvailable
}

func (this *ConnackMessage) SetWildcardSubscriptionAvailable(wildcardSubscriptionAvailable byte) {
	this.wildcardSubscriptionAvailable = wildcardSubscriptionAvailable
	this.dirty = true
}

func (this *ConnackMessage) SubscriptionIdentifierAvailable() byte {
	return this.subscriptionIdentifierAvailable
}

func (this *ConnackMessage) SetSubscriptionIdentifierAvailable(subscriptionIdentifierAvailable byte) {
	this.subscriptionIdentifierAvailable = subscriptionIdentifierAvailable
	this.dirty = true
}

func (this *ConnackMessage) SharedSubscriptionAvailable() byte {
	return this.sharedSubscriptionAvailable
}

func (this *ConnackMessage) SetSharedSubscriptionAvailable(sharedSubscriptionAvailable byte) {
	this.sharedSubscriptionAvailable = sharedSubscriptionAvailable
	this.dirty = true
}

func (this *ConnackMessage) ServerKeepAlive() uint16 {
	return this.serverKeepAlive
}

func (this *ConnackMessage) SetServerKeepAlive(serverKeepAlive uint16) {
	this.serverKeepAlive = serverKeepAlive
	this.dirty = true
}

func (this *ConnackMessage) ResponseInformation() string {
	return this.responseInformation
}

func (this *ConnackMessage) SetResponseInformation(responseInformation string) {
	this.responseInformation = responseInformation
	this.dirty = true
}

func (this *ConnackMessage) ServerReference() string {
	return this.serverReference
}

func (this *ConnackMessage) SetServerReference(serverReference string) {
	this.serverReference = serverReference
	this.dirty = true
}

func (this *ConnackMessage) AuthMethod() string {
	return this.authMethod
}

func (this *ConnackMessage) SetAuthMethod(authMethod string) {
	this.authMethod = authMethod
	this.dirty = true
}

func (this *ConnackMessage) AuthData() []byte {
	return this.authData
}

func (this *ConnackMessage) SetAuthData(authData []byte) {
	this.authData = authData
	this.dirty = true
}

// SessionPresent returns the session present flag value
func (this *ConnackMessage) SessionPresent() bool {
	return this.sessionPresent
}

// SetSessionPresent sets the value of the session present flag
// SetSessionPresent设置会话present标志的值
func (this *ConnackMessage) SetSessionPresent(v bool) {
	if v {
		this.sessionPresent = true
	} else {
		this.sessionPresent = false
	}

	this.dirty = true
}

// ReturnCode returns the return code received for the CONNECT message. The return
// type is an error
func (this *ConnackMessage) ReasonCode() ReasonCode {
	return this.reasonCode
}

func (this *ConnackMessage) SetReasonCode(ret ReasonCode) {
	this.reasonCode = ret
	this.dirty = true
}

func (this *ConnackMessage) Len() int {
	if !this.dirty {
		return len(this.dbuf)
	}

	ml := this.msglen()

	if err := this.SetRemainingLength(int32(ml)); err != nil {
		return 0
	}

	return this.header.msglen() + ml
}

func (this *ConnackMessage) Decode(src []byte) (int, error) {
	total := 0

	n, err := this.header.decode(src)
	total += n
	if err != nil {
		return total, err
	}
	propertyLen := this.header.RemainingLength() // 剩余长度字段
	this.propertiesLen = int(propertyLen)
	oldIndex := total

	b := src[total] // 连接确认标志，7-1必须设置为0

	if b&254 != 0 {
		return 0, ProtocolError // fmt.Errorf("connack/Decode: Bits 7-1 in Connack Acknowledge Flags byte (1) are not 0")
	}

	this.sessionPresent = b&0x1 == 1 // 连接确认标志的第0位为会话存在标志
	total++

	b = src[total] // 连接原因码

	// Read reason code
	if b > UnsupportedWildcardSubscriptions.Value() {
		return 0, ProtocolError // fmt.Errorf("connack/Decode: Invalid CONNACK return code (%d)", b)
	}

	this.reasonCode = ReasonCode(b)
	total++
	// Connack 属性

	if src[total] == SessionExpirationInterval { // 会话过期间隔
		total++
		this.sessionExpiryInterval = binary.BigEndian.Uint32(src[total:])
		total += 4
		if src[total] == SessionExpirationInterval {
			return 0, ProtocolError
		}
	}
	if src[total] == MaximumQuantityReceived { // 接收最大值
		total++
		this.receiveMaximum = binary.BigEndian.Uint16(src[total:])
		total += 2
		if this.receiveMaximum == 0 || src[total] == MaximumQuantityReceived {
			return 0, ProtocolError
		}
	} else {
		this.receiveMaximum = 65535
	}
	if src[total] == MaximumQoS { // 最大服务质量
		total++
		this.maxQos = src[total]
		total++
		if this.maxQos > 2 || this.maxQos < 0 || src[total] == MaximumQoS {
			return 0, ProtocolError
		}
	} else {
		this.maxQos = 2 //  默认2
	}
	if src[total] == PreservePropertyAvailability { // 保留可用
		total++
		this.retainAvailable = src[total]
		total++
		if (this.retainAvailable != 0 && this.retainAvailable != 1) || src[total] == PreservePropertyAvailability {
			return 0, ProtocolError
		}
	} else {
		this.retainAvailable = 0x01
	}
	if src[total] == MaximumMessageLength { // 最大报文长度
		total++
		this.maxPacketSize = binary.BigEndian.Uint32(src[total:])
		total += 4
		if this.maxPacketSize == 0 || src[total] == MaximumMessageLength {
			return 0, ProtocolError
		}
	} else {
		// TODO 按照协议由固定报头中的剩余长度可编码最大值和协议报头对数据包的大小做限制
	}
	if src[total] == AssignCustomerIdentifiers { // 分配的客户端标识符
		total++
		nextLen := oldIndex + this.propertiesLen - total
		i := 0
		for ; i < nextLen; i++ {
			switch src[total+i] {
			case AssignCustomerIdentifiers:
				return 0, ProtocolError
			case MaximumLengthOfTopicAlias, ReasonString, UserProperty, WildcardSubscriptionAvailability,
				AvailabilityOfSubscriptionIdentifiers, SharedSubscriptionAvailability, ServerSurvivalTime,
				SolicitedMessage, ServerReference, AuthenticationMethod, AuthenticationData:
				goto ACI
			}
		}
	ACI:
		if i == 0 {
			return 0, ProtocolError
		}
		this.assignedIdentifier = string(src[total : total+i])
		total += i
		if i == nextLen {
			this.dirty = false
			return total, nil
		}
	}
	if src[total] == MaximumLengthOfTopicAlias { // 主题别名最大值
		total++
		this.topicAliasMax = binary.BigEndian.Uint16(src[total:])
		total += 2
		if src[total] == MaximumLengthOfTopicAlias {
			return 0, ProtocolError
		}
	}
	if src[total] == ReasonString { // 分配的客户端标识符
		total++
		nextLen := oldIndex + this.propertiesLen - total
		i := 0
		for ; i < nextLen; i++ {
			switch src[total+i] {
			case ReasonString:
				return 0, ProtocolError
			case UserProperty, WildcardSubscriptionAvailability,
				AvailabilityOfSubscriptionIdentifiers, SharedSubscriptionAvailability, ServerSurvivalTime,
				SolicitedMessage, ServerReference, AuthenticationMethod, AuthenticationData:
				goto RS
			}
		}
	RS:
		if i == 0 {
			return 0, ProtocolError
		}
		this.reasonStr = string(src[total : total+i])
		total += i
		if i == nextLen {
			this.dirty = false
			return total, nil
		}
	}
	if src[total] == UserProperty { // 用户属性
		total++
		// TODO 这里默认为 ':' 为分隔符
		i, nextLen := 0, oldIndex+this.propertiesLen-total
		if nextLen <= 1 {
			return total, ProtocolError
		}
		splitTag := 0
		this.userProperties = make(map[string]string)
		for ; i < nextLen; i++ {
			switch src[total+i] {
			case ':':
				splitTag = i // 记录分隔符位置
			case UserProperty:
				if splitTag == 0 || splitTag == nextLen-1 {
					// 遇到了下一个用户属性都还没有遇到分隔符
					// 或者最后一个字节才是分隔符，协议错误
					return total, ProtocolError
				}
				this.userProperties[string(src[total:total+splitTag])] = string(src[total+splitTag+1 : total+i])
				splitTag = 0
			case AuthenticationMethod:
				if splitTag > 0 && splitTag != nextLen-1 {
					this.userProperties[string(src[total:total+splitTag])] = string(src[total+splitTag+1 : total+i])
					splitTag = 0
				} else if splitTag == 0 || splitTag == nextLen-1 {
					// 遇到了下一个用户属性都还没有遇到分隔符
					// 或者最后一个字节才是分隔符，协议错误
					return total, ProtocolError
				}
				if len(this.userProperties) == 0 { // 没有用户属性，协议错误
					return total, ProtocolError
				}
				goto PP
			case AuthenticationData: // 没有遇到认证方法，却遇到了认证数据，协议错误
				return total, ProtocolError
			}
		}
	PP:
		if i == 0 {
			return total, ProtocolError
		} else if i == nextLen {
			if splitTag == 0 || splitTag == nextLen-1 {
				// 属性数据都到最后一个字节了，都没有分隔符
				// 或者最后一个字节才是分隔符，协议错误
				return total, ProtocolError
			}
			this.userProperties[string(src[total:total+splitTag])] = string(src[total+splitTag+1 : total+i])
		}
		total += i
	}
	if src[total] == WildcardSubscriptionAvailability { // 通配符订阅可用
		total++
		this.wildcardSubscriptionAvailable = src[total]
		total++
		if (this.wildcardSubscriptionAvailable != 0 && this.wildcardSubscriptionAvailable != 1) ||
			src[total] == WildcardSubscriptionAvailability {
			return 0, ProtocolError
		}
	} else {
		this.wildcardSubscriptionAvailable = 0x01
	}
	if src[total] == AvailabilityOfSubscriptionIdentifiers { // 订阅标识符可用
		total++
		this.subscriptionIdentifierAvailable = src[total]
		total++
		if (this.subscriptionIdentifierAvailable != 0 && this.subscriptionIdentifierAvailable != 1) ||
			src[total] == AvailabilityOfSubscriptionIdentifiers {
			return 0, ProtocolError
		}
	} else {
		this.subscriptionIdentifierAvailable = 0x01
	}
	if src[total] == SharedSubscriptionAvailability { // 共享订阅标识符可用
		total++
		this.sharedSubscriptionAvailable = src[total]
		total++
		if (this.sharedSubscriptionAvailable != 0 && this.sharedSubscriptionAvailable != 1) ||
			src[total] == SharedSubscriptionAvailability {
			return 0, ProtocolError
		}
	} else {
		this.subscriptionIdentifierAvailable = 0x01
	}
	if src[total] == ServerSurvivalTime { // 服务保持连接
		total++
		this.serverKeepAlive = binary.BigEndian.Uint16(src[total:])
		total++
		if src[total] == ServerSurvivalTime {
			return 0, ProtocolError
		}
	}
	if src[total] == SolicitedMessage { // 响应信息
		total++
		nextLen := oldIndex + this.propertiesLen - total
		i := 0
		for ; i < nextLen; i++ {
			switch src[total+i] {
			case SolicitedMessage:
				return 0, ProtocolError
			case ServerReference, AuthenticationMethod, AuthenticationData:
				goto SM
			}
		}
	SM:
		if i == 0 {
			return 0, ProtocolError
		}
		this.responseInformation = string(src[total : total+i])
		total += i
		if i == nextLen {
			this.dirty = false
			return total, nil
		}
	}
	if src[total] == ServerReference { // 服务端参考
		total++
		nextLen := oldIndex + this.propertiesLen - total
		i := 0
		for ; i < nextLen; i++ {
			switch src[total+i] {
			case ServerReference:
				return 0, ProtocolError
			case AuthenticationMethod, AuthenticationData:
				goto SR
			}
		}
	SR:
		if i == 0 {
			return 0, ProtocolError
		}
		this.serverReference = string(src[total : total+i])
		total += i
		if i == nextLen {
			this.dirty = false
			return total, nil
		}
	}
	if src[total] == AuthenticationMethod { // 认证方法
		total++
		nextLen := oldIndex + this.propertiesLen - total
		i := 0
		for ; i < nextLen; i++ {
			switch src[total+i] {
			case AuthenticationMethod:
				return 0, ProtocolError
			case AuthenticationData:
				goto AM
			}
		}
	AM:
		if i == 0 {
			return 0, ProtocolError
		}
		this.authMethod = string(src[total : total+i])
		total += i
		if i == nextLen {
			this.dirty = false
			return total, nil
		}
	}
	if src[total] == AuthenticationData { // 认证数据
		total++
		nextLen := oldIndex + this.propertiesLen - total
		i := 0
		for ; i < nextLen; i++ {
			switch src[total+i] {
			case AuthenticationData:
				return 0, ProtocolError
			}
		}
		if i == 0 {
			return 0, ProtocolError
		}
		this.authData = src[total : total+i]
		total += i
	}
	this.dirty = false

	return total, nil
}

func (this *ConnackMessage) Encode(dst []byte) (int, error) {
	this.build()
	if !this.dirty {
		if len(dst) < len(this.dbuf) {
			return 0, fmt.Errorf("connack/Encode: Insufficient buffer size. Expecting %d, got %d.", len(this.dbuf), len(dst))
		}

		return copy(dst, this.dbuf), nil
	}

	// CONNACK remaining length fixed at 2 bytes
	hl := this.header.msglen()
	ml := this.msglen()

	if len(dst) < hl+ml {
		return 0, fmt.Errorf("connack/Encode: Insufficient buffer size. Expecting %d, got %d.", hl+ml, len(dst))
	}

	if err := this.SetRemainingLength(int32(ml)); err != nil {
		return 0, err
	}

	total := 0

	n, err := this.header.encode(dst[total:])
	total += n
	if err != nil {
		return 0, err
	}

	if this.sessionPresent { // 连接确认标志
		dst[total] = 1
	}
	total++

	if this.reasonCode > UnsupportedWildcardSubscriptions {
		return total, fmt.Errorf("connack/Encode: Invalid CONNACK return code (%d)", this.reasonCode)
	}

	dst[total] = this.reasonCode.Value() // 原因码
	total++

	// 属性
	if this.sessionExpiryInterval > 0 { // 会话过期间隔
		dst[total] = SessionExpirationInterval
		total++
		binary.BigEndian.PutUint32(dst[total:], this.sessionExpiryInterval)
		total += 4
	}
	if this.receiveMaximum > 0 && this.receiveMaximum < 65535 { // 接收最大值
		dst[total] = MaximumQuantityReceived
		total++
		binary.BigEndian.PutUint16(dst[total:], this.receiveMaximum)
		total += 2
	}
	if this.maxQos > 0 { // 最大服务质量，正常都会编码
		dst[total] = MaximumQoS
		total++
		dst[total] = this.maxQos
		total++
	}
	if this.retainAvailable != 1 { // 保留可用
		dst[total] = PreservePropertyAvailability
		total++
		dst[total] = this.retainAvailable
		total++
	}
	if this.maxPacketSize != 1 { // 最大报文长度
		dst[total] = MaximumMessageLength
		total++
		binary.BigEndian.PutUint32(dst[total:], this.maxPacketSize)
		total += 4
	}
	if len(this.assignedIdentifier) > 0 { // 分配客户标识符
		dst[total] = AssignCustomerIdentifiers
		total++
		copy(dst[total:], this.assignedIdentifier)
		total += len(this.assignedIdentifier)
	}
	if this.topicAliasMax > 0 { // 主题别名最大值
		dst[total] = MaximumLengthOfTopicAlias
		total++
		binary.BigEndian.PutUint16(dst[total:], this.topicAliasMax)
		total += 2
	}
	if len(this.reasonStr) > 0 { // 原因字符串
		dst[total] = ReasonString
		total++
		copy(dst[total:], this.reasonStr)
		total += len(this.reasonStr)
	}
	for k, v := range this.userProperties { // 用户属性
		dst[total] = UserProperty
		total++
		copy(dst[total:], k)
		total += len(k)
		dst[total] = ':'
		total++
		copy(dst[total:], v)
		total += len(v)
	}
	if this.wildcardSubscriptionAvailable != 1 { // 通配符订阅可用
		dst[total] = WildcardSubscriptionAvailability
		total++
		dst[total] = this.wildcardSubscriptionAvailable
		total++
	}
	if this.subscriptionIdentifierAvailable != 1 { // 订阅标识符可用
		dst[total] = AvailabilityOfSubscriptionIdentifiers
		total++
		dst[total] = this.subscriptionIdentifierAvailable
		total++
	}
	if this.sharedSubscriptionAvailable != 1 { // 共享订阅可用
		dst[total] = SharedSubscriptionAvailability
		total++
		dst[total] = this.sharedSubscriptionAvailable
		total++
	}
	if this.serverKeepAlive > 0 { // 服务端保持连接
		dst[total] = ServerSurvivalTime
		total++
		binary.BigEndian.PutUint16(dst[total:], this.serverKeepAlive)
		total += 2
	}
	if len(this.responseInformation) > 0 { // 响应信息
		dst[total] = SolicitedMessage
		total++
		copy(dst[total:], this.responseInformation)
		total += len(this.responseInformation)
	}
	if len(this.serverReference) > 0 { // 服务端参考
		dst[total] = ServerReference
		total++
		copy(dst[total:], this.serverReference)
		total += len(this.serverReference)
	}
	if len(this.authMethod) > 0 { // 认证方法
		dst[total] = AuthenticationMethod
		total++
		copy(dst[total:], this.authMethod)
		total += len(this.authMethod)
	}
	if len(this.authData) > 0 { // 认证数据
		dst[total] = AuthenticationData
		total++
		copy(dst[total:], this.authData)
		total += len(this.authData)
	}
	return total, nil
}

// propertiesLen
func (this *ConnackMessage) msglen() int {
	return this.propertiesLen
}
