package messagev5

import (
	"encoding/binary"
	"fmt"
	"regexp"
)

var (
	clientIdRegexp *regexp.Regexp
	defaultCId     = []byte("#")
)

func init() {
	// Added space for Paho compliance test
	// Added underscore (_) for MQTT C client test
	clientIdRegexp = regexp.MustCompile("^[/\\-0-9a-zA-Z _]*$")
}

// ConnectMessage After a Network Connection is established by a Client to a Server, the first Packet
// sent from the Client to the Server MUST be a CONNECT Packet [MQTT-3.1.0-1].
//
// A Client can only send the CONNECT Packet once over a Network Connection. The Server
// MUST process a second CONNECT Packet sent from a Client as a protocol violation and
// disconnect the Client [MQTT-3.1.0-2].  See section 4.8 for information about
// handling errors.
type ConnectMessage struct {
	header

	// 7: username flag
	// 6: password flag
	// 5: will retain
	// 4-3: will QoS
	// 2: will flag
	// 1: clean session
	// 0: reserved
	connectFlags byte

	version byte

	keepAlive uint16

	protoName,
	clientId,
	willTopic,
	willMessage,
	username,
	password []byte

	propertiesLen         uint32   // 属性长度
	sessionExpiryInterval uint32   // 会话过期间隔
	receiveMaximum        uint16   // 接收最大值
	maxPacketSize         uint32   // 最大报文长度是 MQTT 控制报文的总长度
	topicAliasMax         uint16   // 主题别名最大值（Topic Alias Maximum）
	requestRespInfo       byte     // 一个字节表示的 0 或 1, 请求响应信息
	requestProblemInfo    byte     // 一个字节表示的 0 或 1 , 请求问题信息
	userProperty          [][]byte // 用户属性，可变报头的
	authMethod            []byte   // 认证方法
	authData              []byte   // 认证数据

	willPropertiesLen      uint32   // 遗嘱属性长度
	willDelayInterval      uint32   // 遗嘱延时间隔
	payloadFormatIndicator byte     // 载荷格式指示
	willMsgExpiryInterval  uint32   // 遗嘱消息过期间隔
	contentType            []byte   // 遗嘱内容类型 UTF-8 格式编码
	willUserProperty       [][]byte // 用户属性，载荷遗嘱属性的
	responseTopic          []byte   // 响应主题
	correlationData        []byte   // 对比数据
}

var _ Message = (*ConnectMessage)(nil)

// NewConnectMessage creates a new CONNECT message.
func NewConnectMessage() *ConnectMessage {
	msg := &ConnectMessage{}
	_ = msg.SetType(CONNECT)
	msg.SetRequestProblemInfo(1)
	msg.SetMaxPacketSize(1024)
	return msg
}

// String returns a string representation of the CONNECT message
func (this ConnectMessage) String() string {
	return fmt.Sprintf("Header==>> %s, Variable header==>> \tProtocol Name=%s\tProtocol Version=%v, "+
		"Connect Flags=%08b\t"+
		"User Name Flag=%v\tPassword Flag=%v\tWill Retain=%v\tWill QoS=%d\tWill Flag=%v\tClean Start=%v\tReserved=%v, "+
		"KeepAlive=%v, "+
		"Properties\t"+
		"Length=%v\tSession Expiry Interval=%v\tReceive Maximum=%v\tMaximum Packet Size=%v\t"+
		"Topic Alias Maximum=%v\tRequest Response Information=%v\tRequest Problem Information=%v\t"+
		"User Property=%s\tAuthentication Method=%s\tAuthentication Data=%s, "+
		"载荷==>>"+
		"Client ID=%q, "+
		"Will Properties\t"+
		"Will Properties Len=%v\tWill Delay Interval=%v\tPayload Format Indicator=%v\tMessage Expiry Interval=%v\t"+
		"Content Type=%s\tResponse Topic=%s\tCorrelation Data=%v\tUser Property=%s, "+
		"Will Topic=%s\tWill Payload=%s\tUsername=%s\n\t\tPassword=%s",
		this.header,

		this.protoName,
		this.Version(),
		this.connectFlags,
		this.UsernameFlag(),
		this.PasswordFlag(),
		this.WillRetain(),
		this.WillQos(),
		this.WillFlag(),
		this.CleanSession(),
		this.connectFlags&0x01,

		this.KeepAlive(),

		this.propertiesLen,
		this.sessionExpiryInterval,
		this.receiveMaximum,
		this.maxPacketSize,
		this.topicAliasMax,
		this.requestRespInfo,
		this.requestProblemInfo,
		this.userProperty,
		this.authMethod,
		this.authData,

		this.ClientId(),
		this.willPropertiesLen,
		this.willDelayInterval,
		this.payloadFormatIndicator,
		this.willMsgExpiryInterval,
		this.contentType,
		this.responseTopic,
		this.correlationData,
		this.willUserProperty,

		this.WillTopic(),
		this.WillMessage(),
		this.Username(),
		this.Password(),
	)
}

// 自动设置remlen , 属性长度， 遗嘱属性长度
func (this *ConnectMessage) build() {
	remlen := 0
	remlen += 1 // version
	remlen += 1 // 连接标志
	remlen += 2 // keep alive
	//  protoName,
	//	clientId,
	//	willTopic,
	//	willMessage,
	//	username,
	//	password []byte
	remlen += 2
	remlen += len(this.protoName)
	// 载荷 客户端标识符
	remlen += 2
	remlen += len(this.clientId)
	// 载荷 遗嘱
	if this.WillFlag() && len(this.willTopic) > 0 {
		remlen += 2
		remlen += len(this.willTopic)
	}
	if this.WillFlag() && len(this.willMessage) > 0 {
		remlen += 2
		remlen += len(this.willMessage)
	}
	if this.UsernameFlag() {
		remlen += 2
		remlen += len(this.username)
	}
	if this.PasswordFlag() {
		remlen += 2
		remlen += len(this.password)
	}
	// 属性长度
	//  sessionExpiryInterval uint32            // 会话过期间隔
	//	receiveMaximum        uint16            // 接收最大值
	//	maxPacketSize         uint32            // 最大报文长度是 MQTT 控制报文的总长度
	//	topicAliasMax         uint16            // 主题别名最大值（Topic Alias Maximum）
	//	requestRespInfo       byte              // 一个字节表示的 0 或 1, 请求响应信息
	//	requestProblemInfo    byte              // 一个字节表示的 0 或 1 , 请求问题信息
	//	userProperty          map[string]string // 用户属性，可变报头的
	//	authMethod            string            // 认证方法
	//	authData              []byte            // 认证数据
	propertiesLen := 0
	if this.sessionExpiryInterval > 0 {
		propertiesLen += 5
	}
	if this.receiveMaximum > 0 && this.receiveMaximum < 65535 {
		propertiesLen += 3
	}
	if this.maxPacketSize > 0 {
		propertiesLen += 5
	}
	if this.topicAliasMax > 0 {
		propertiesLen += 3
	}
	if this.requestRespInfo != 0 {
		propertiesLen += 2
	}
	if this.requestProblemInfo != 1 {
		propertiesLen += 2
	}
	for _, v := range this.userProperty {
		propertiesLen += 1
		propertiesLen += 2
		propertiesLen += len(v)
	}
	if len(this.authMethod) > 0 {
		propertiesLen += 1
		propertiesLen += 2
		propertiesLen += len(this.authMethod)
	}
	if len(this.authData) > 0 {
		propertiesLen += 1
		propertiesLen += 2
		propertiesLen += len(this.authData)
	}
	this.propertiesLen = uint32(propertiesLen)

	//	willDelayInterval      uint32            // 遗嘱延时间隔
	//	payloadFormatIndicator byte              // 载荷格式指示
	//	willMsgExpiryInterval  uint32            // 遗嘱消息过期间隔
	//	contentType            string            // 遗嘱内容类型 UTF-8 格式编码
	//	willUserProperty       map[string]string // 用户属性，载荷遗嘱属性的
	//	responseTopic          string            // 响应主题
	//	correlationData        []byte            // 对比数据
	// 载荷 遗嘱属性
	willPropertiesLen := 0
	if this.willDelayInterval > 0 {
		willPropertiesLen += 5
	}
	if this.payloadFormatIndicator == 1 {
		willPropertiesLen += 2
	}
	if this.willMsgExpiryInterval > 0 {
		willPropertiesLen += 5
	}
	if len(this.contentType) > 0 {
		willPropertiesLen += 1
		willPropertiesLen += 2
		willPropertiesLen += len(this.contentType)
	}
	for _, v := range this.willUserProperty {
		willPropertiesLen += 1
		willPropertiesLen += 2
		willPropertiesLen += len(v)
	}
	if len(this.responseTopic) > 0 {
		willPropertiesLen += 1
		willPropertiesLen += 2
		willPropertiesLen += len(this.responseTopic)
	}
	if len(this.correlationData) > 0 {
		willPropertiesLen += 1
		willPropertiesLen += 2
		willPropertiesLen += len(this.correlationData)
	}
	this.willPropertiesLen = uint32(willPropertiesLen)
	_ = this.SetRemainingLength(int32(remlen + propertiesLen + len(lbEncode(this.propertiesLen)) + willPropertiesLen + len(lbEncode(this.willPropertiesLen))))
}
func (this *ConnectMessage) PropertiesLen() uint32 {
	return this.propertiesLen
}

func (this *ConnectMessage) SetPropertiesLen(propertiesLen uint32) {
	this.propertiesLen = propertiesLen
	this.dirty = true
}

func (this *ConnectMessage) SessionExpiryInterval() uint32 {
	return this.sessionExpiryInterval
}

func (this *ConnectMessage) SetSessionExpiryInterval(sessionExpiryInterval uint32) {
	this.sessionExpiryInterval = sessionExpiryInterval
	this.dirty = true
}

func (this *ConnectMessage) ReceiveMaximum() uint16 {
	return this.receiveMaximum
}

func (this *ConnectMessage) SetReceiveMaximum(receiveMaximum uint16) {
	this.receiveMaximum = receiveMaximum
	this.dirty = true
}

func (this *ConnectMessage) MaxPacketSize() uint32 {
	return this.maxPacketSize
}

func (this *ConnectMessage) SetMaxPacketSize(maxPacketSize uint32) {
	this.maxPacketSize = maxPacketSize
	this.dirty = true
}

func (this *ConnectMessage) TopicAliasMax() uint16 {
	return this.topicAliasMax
}

func (this *ConnectMessage) SetTopicAliasMax(topicAliasMax uint16) {
	this.topicAliasMax = topicAliasMax
	this.dirty = true
}

func (this *ConnectMessage) RequestRespInfo() byte {
	return this.requestRespInfo
}

func (this *ConnectMessage) SetRequestRespInfo(requestRespInfo byte) {
	this.requestRespInfo = requestRespInfo
	this.dirty = true
}

func (this *ConnectMessage) RequestProblemInfo() byte {
	return this.requestProblemInfo
}

func (this *ConnectMessage) SetRequestProblemInfo(requestProblemInfo byte) {
	this.requestProblemInfo = requestProblemInfo
	this.dirty = true
}

func (this *ConnectMessage) UserProperty() [][]byte {
	return this.userProperty
}

func (this *ConnectMessage) AddUserPropertys(userProperty [][]byte) {
	this.userProperty = append(this.userProperty, userProperty...)
	this.dirty = true
}
func (this *ConnectMessage) AddUserProperty(userProperty []byte) {
	this.userProperty = append(this.userProperty, userProperty)
	this.dirty = true
}

func (this *ConnectMessage) WillUserProperty() [][]byte {
	return this.willUserProperty
}

func (this *ConnectMessage) AddWillUserPropertys(willUserProperty [][]byte) {
	this.willUserProperty = append(this.willUserProperty, willUserProperty...)
	this.dirty = true
}
func (this *ConnectMessage) AddWillUserProperty(willUserProperty []byte) {
	this.willUserProperty = append(this.willUserProperty, willUserProperty)
	this.dirty = true
}
func (this *ConnectMessage) AuthMethod() []byte {
	return this.authMethod
}

func (this *ConnectMessage) SetAuthMethod(authMethod []byte) {
	this.authMethod = authMethod
	this.dirty = true
}

func (this *ConnectMessage) AuthData() []byte {
	return this.authData
}

func (this *ConnectMessage) SetAuthData(authData []byte) {
	this.authData = authData
	this.dirty = true
}

func (this *ConnectMessage) WillPropertiesLen() uint32 {
	return this.willPropertiesLen
}

func (this *ConnectMessage) SetWillPropertiesLen(willPropertiesLen uint32) {
	this.willPropertiesLen = willPropertiesLen
	this.dirty = true
}

func (this *ConnectMessage) WillDelayInterval() uint32 {
	return this.willDelayInterval
}

func (this *ConnectMessage) SetWillDelayInterval(willDelayInterval uint32) {
	this.willDelayInterval = willDelayInterval
	this.dirty = true
}

func (this *ConnectMessage) PayloadFormatIndicator() byte {
	return this.payloadFormatIndicator
}

func (this *ConnectMessage) SetPayloadFormatIndicator(payloadFormatIndicator byte) {
	this.payloadFormatIndicator = payloadFormatIndicator
	this.dirty = true
}

func (this *ConnectMessage) WillMsgExpiryInterval() uint32 {
	return this.willMsgExpiryInterval
}

func (this *ConnectMessage) SetWillMsgExpiryInterval(willMsgExpiryInterval uint32) {
	this.willMsgExpiryInterval = willMsgExpiryInterval
	this.dirty = true
}

func (this *ConnectMessage) ContentType() []byte {
	return this.contentType
}

func (this *ConnectMessage) SetContentType(contentType []byte) {
	this.contentType = contentType
	this.dirty = true
}

func (this *ConnectMessage) ResponseTopic() []byte {
	return this.responseTopic
}

func (this *ConnectMessage) SetResponseTopic(responseTopic []byte) {
	this.responseTopic = responseTopic
	this.dirty = true
}

func (this *ConnectMessage) CorrelationData() []byte {
	return this.correlationData
}

func (this *ConnectMessage) SetCorrelationData(correlationData []byte) {
	this.correlationData = correlationData
	this.dirty = true
}

// Version returns the the 8 bit unsigned value that represents the revision level
// of the protocol used by the Client. The value of the Protocol Level field for
// the version 3.1.1 of the protocol is 4 (0x04).
func (this *ConnectMessage) Version() byte {
	return this.version
}

// SetVersion sets the version value of the CONNECT message
func (this *ConnectMessage) SetVersion(v byte) error {
	if pn, ok := SupportedVersions[v]; !ok {
		return fmt.Errorf("connect/SetVersion: Invalid version number %d", v)
	} else {
		this.protoName = []byte(pn)
	}

	this.version = v
	this.dirty = true

	return nil
}

// CleanSession returns the bit that specifies the handling of the Session state.
// The Client and Server can store Session state to enable reliable messaging to
// continue across a sequence of Network Connections. This bit is used to control
// the lifetime of the Session state.
// CleanSession返回指定会话状态处理的位。
//客户端和服务器可以存储会话状态，以实现可靠的消息传递
//继续通过网络连接序列。这个位用来控制
//会话状态的生存期。
func (this *ConnectMessage) CleanSession() bool {
	return ((this.connectFlags >> 1) & 0x1) == 1
}

// SetCleanSession sets the bit that specifies the handling of the Session state.
func (this *ConnectMessage) SetCleanSession(v bool) {
	if v {
		this.connectFlags |= 0x2 // 00000010
	} else {
		this.connectFlags &= 253 // 11111101
	}

	this.dirty = true
}

// WillFlag returns the bit that specifies whether a Will Message should be stored
// on the server. If the Will Flag is set to 1 this indicates that, if the Connect
// request is accepted, a Will Message MUST be stored on the Server and associated
// with the Network Connection.
// WillFlag返回指定是否存储Will消息的位
//在服务器上。如果Will标志设置为1，这表示如果连接
//请求被接受，一个Will消息必须存储在服务器上并关联
//与网络连接。
func (this *ConnectMessage) WillFlag() bool {
	return ((this.connectFlags >> 2) & 0x1) == 1
}

// SetWillFlag sets the bit that specifies whether a Will Message should be stored
// on the server.
func (this *ConnectMessage) SetWillFlag(v bool) {
	if v {
		this.connectFlags |= 0x4 // 00000100
	} else {
		this.connectFlags &= 251 // 11111011
	}

	this.dirty = true
}

// WillQos returns the two bits that specify the QoS level to be used when publishing
// the Will Message.
func (this *ConnectMessage) WillQos() byte {
	return (this.connectFlags >> 3) & 0x3
}

// SetWillQos sets the two bits that specify the QoS level to be used when publishing
// the Will Message.
func (this *ConnectMessage) SetWillQos(qos byte) error {
	if qos != QosAtMostOnce && qos != QosAtLeastOnce && qos != QosExactlyOnce {
		return fmt.Errorf("connect/SetWillQos: Invalid QoS level %d", qos)
	}

	this.connectFlags = (this.connectFlags & 231) | (qos << 3) // 231 = 11100111
	this.dirty = true

	return nil
}

// WillRetain returns the bit specifies if the Will Message is to be Retained when it
// is published.
// Will retain返回指定Will消息是否被保留的位
//出版。
func (this *ConnectMessage) WillRetain() bool {
	return ((this.connectFlags >> 5) & 0x1) == 1
}

// SetWillRetain sets the bit specifies if the Will Message is to be Retained when it
// is published.
func (this *ConnectMessage) SetWillRetain(v bool) {
	if v {
		this.connectFlags |= 32 // 00100000
	} else {
		this.connectFlags &= 223 // 11011111
	}

	this.dirty = true
}

// UsernameFlag returns the bit that specifies whether a user name is present in the
// payload.
func (this *ConnectMessage) UsernameFlag() bool {
	return ((this.connectFlags >> 7) & 0x1) == 1
}

// SetUsernameFlag sets the bit that specifies whether a user name is present in the
// payload.
func (this *ConnectMessage) SetUsernameFlag(v bool) {
	if v {
		this.connectFlags |= 128 // 10000000
	} else {
		this.connectFlags &= 127 // 01111111
	}

	this.dirty = true
}

// PasswordFlag returns the bit that specifies whether a password is present in the
// payload.
func (this *ConnectMessage) PasswordFlag() bool {
	return ((this.connectFlags >> 6) & 0x1) == 1
}

// SetPasswordFlag sets the bit that specifies whether a password is present in the
// payload.
func (this *ConnectMessage) SetPasswordFlag(v bool) {
	if v {
		this.connectFlags |= 64 // 01000000
	} else {
		this.connectFlags &= 191 // 10111111
	}

	this.dirty = true
}

// KeepAlive returns a time interval measured in seconds. Expressed as a 16-bit word,
// it is the maximum time interval that is permitted to elapse between the point at
// which the Client finishes transmitting one Control Packet and the point it starts
// sending the next.
func (this *ConnectMessage) KeepAlive() uint16 {
	return this.keepAlive
}

// SetKeepAlive sets the time interval in which the server should keep the connection
// alive.
func (this *ConnectMessage) SetKeepAlive(v uint16) {
	this.keepAlive = v

	this.dirty = true
}

// ClientId returns an ID that identifies the Client to the Server. Each Client
// connecting to the Server has a unique ClientId. The ClientId MUST be used by
// Clients and by Servers to identify state that they hold relating to this MQTT
// Session between the Client and the Server
func (this *ConnectMessage) ClientId() []byte {
	return this.clientId
}

// SetClientId sets an ID that identifies the Client to the Server.
func (this *ConnectMessage) SetClientId(v []byte) error {
	if len(v) > 0 && !this.validClientId(v) {
		return InvalidTopicName
	}

	this.clientId = v
	this.dirty = true

	return nil
}

// WillTopic returns the topic in which the Will Message should be published to.
// If the Will Flag is set to 1, the Will Topic must be in the payload.
func (this *ConnectMessage) WillTopic() []byte {
	return this.willTopic
}

// SetWillTopic sets the topic in which the Will Message should be published to.
func (this *ConnectMessage) SetWillTopic(v []byte) {
	this.willTopic = v

	if len(v) > 0 {
		this.SetWillFlag(true)
	} else if len(this.willMessage) == 0 {
		this.SetWillFlag(false)
	}

	this.dirty = true
}

// WillMessage returns the Will Message that is to be published to the Will Topic.
func (this *ConnectMessage) WillMessage() []byte {
	return this.willMessage
}

// SetWillMessage sets the Will Message that is to be published to the Will Topic.
func (this *ConnectMessage) SetWillMessage(v []byte) {
	this.willMessage = v

	if len(v) > 0 {
		this.SetWillFlag(true)
	} else if len(this.willTopic) == 0 {
		this.SetWillFlag(false)
	}

	this.dirty = true
}

// Username returns the username from the payload. If the User Name Flag is set to 1,
// this must be in the payload. It can be used by the Server for authentication and
// authorization.
func (this *ConnectMessage) Username() []byte {
	return this.username
}

// SetUsername sets the username for authentication.
func (this *ConnectMessage) SetUsername(v []byte) {
	this.username = v

	if len(v) > 0 {
		this.SetUsernameFlag(true)
	} else {
		this.SetUsernameFlag(false)
	}

	this.dirty = true
}

// Password returns the password from the payload. If the Password Flag is set to 1,
// this must be in the payload. It can be used by the Server for authentication and
// authorization.
func (this *ConnectMessage) Password() []byte {
	return this.password
}

// SetPassword sets the username for authentication.
func (this *ConnectMessage) SetPassword(v []byte) {
	this.password = v

	if len(v) > 0 {
		this.SetPasswordFlag(true)
	} else {
		this.SetPasswordFlag(false)
	}

	this.dirty = true
}

func (this *ConnectMessage) Len() int {
	if !this.dirty {
		return len(this.dbuf)
	}

	ml := this.msglen()

	if err := this.SetRemainingLength(int32(ml)); err != nil {
		return 0
	}

	return this.header.msglen() + ml
}

// Decode For the CONNECT message, the error returned could be a ConnackReturnCode, so
// be sure to check that. Otherwise it's a generic error. If a generic error is
// returned, this Message should be considered invalid.
//
// Caller should call ValidConnackError(err) to see if the returned error is
// a Connack error. If so, caller should send the Client back the corresponding
// CONNACK message.
func (this *ConnectMessage) Decode(src []byte) (int, error) {
	total := 0

	n, err := this.header.decode(src[total:])
	if err != nil {
		return total + n, err
	}
	total += n

	if n, err = this.decodeMessage(src[total:]); err != nil {
		return total + n, err
	}
	total += n

	this.dirty = false

	return total, nil
}

func (this *ConnectMessage) Encode(dst []byte) (int, error) {
	if !this.dirty {
		if len(dst) < len(this.dbuf) {
			return 0, fmt.Errorf("connect/Encode: Insufficient buffer size. Expecting %d, got %d.", len(this.dbuf), len(dst))
		}

		return copy(dst, this.dbuf), nil
	}

	if this.Type() != CONNECT {
		return 0, fmt.Errorf("connect/Encode: Invalid message type. Expecting %d, got %d", CONNECT, this.Type())
	}

	_, ok := SupportedVersions[this.version]
	if !ok {
		return 0, UnSupportedProtocolVersion
	}

	ml := this.msglen()
	hl := this.header.msglen()

	if len(dst) < hl+ml {
		return 0, fmt.Errorf("connect/Encode: Insufficient buffer size. Expecting %d, got %d.", hl+ml, len(dst))
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

	n, err = this.encodeMessage(dst[total:])
	total += n
	if err != nil {
		return total, err
	}

	return total, nil
}

func (this *ConnectMessage) encodeMessage(dst []byte) (int, error) {
	total := 0
	/**
		===可变报头===
	**/
	n, err := writeLPBytes(dst[total:], []byte(SupportedVersions[this.version])) // 写入协议长度和协议名称
	total += n
	if err != nil {
		return total, err
	}

	dst[total] = this.version // 写入协议版本号
	total += 1

	dst[total] = this.connectFlags // 写入连接标志
	total += 1

	binary.BigEndian.PutUint16(dst[total:], this.keepAlive) // 写入保持连接
	total += 2

	// 属性
	pLen := lbEncode(this.propertiesLen) // 属性长度
	copy(dst[total:], pLen)
	total += len(pLen)

	if this.sessionExpiryInterval > 0 {
		dst[total] = SessionExpirationInterval
		total++
		binary.BigEndian.PutUint32(dst[total:], this.sessionExpiryInterval) // 会话过期间隔
		total += 4
	}

	if this.receiveMaximum > 0 && this.receiveMaximum < 65535 {
		dst[total] = MaximumQuantityReceived
		total++
		binary.BigEndian.PutUint16(dst[total:], this.receiveMaximum) // 接收最大值
		total += 2
	}

	dst[total] = MaximumMessageLength
	total++
	binary.BigEndian.PutUint32(dst[total:], this.maxPacketSize) // 最大报文长度
	total += 4

	if this.topicAliasMax > 0 {
		dst[total] = MaximumLengthOfTopicAlias
		total++
		binary.BigEndian.PutUint16(dst[total:], this.topicAliasMax) // 主题别名最大值
		total += 2
	}

	// TODO if this.requestRespInfo == 0 ==>> 可发可不发
	if this.requestRespInfo != 0 { // 默认0
		dst[total] = RequestResponseInformation
		total++
		dst[total] = this.requestRespInfo // 请求响应信息
		total++
	}
	if this.requestProblemInfo != 1 { // 默认1
		dst[total] = RequestProblemInformation
		total++
		dst[total] = this.requestProblemInfo // 请求问题信息
		total++
	}

	for _, v1 := range this.userProperty {
		dst[total] = UserProperty // 用户属性
		total++
		n, err = writeLPBytes(dst[total:], v1)
		total += n
		if err != nil {
			return total, err
		}
	}

	if len(this.authMethod) > 0 {
		dst[total] = AuthenticationMethod // 认证方法
		total++
		n, err = writeLPBytes(dst[total:], this.authMethod)
		total += n
		if err != nil {
			return total, err
		}
	}
	if len(this.authData) > 0 {
		dst[total] = AuthenticationData // 认证数据
		total++
		n, err = writeLPBytes(dst[total:], this.authData)
		total += n
		if err != nil {
			return total, err
		}
	}

	/**
		===载荷===
	**/
	n, err = writeLPBytes(dst[total:], this.clientId) // 客户标识符
	total += n
	if err != nil {
		return total, err
	}
	if this.WillFlag() {
		// 遗嘱属性
		wpLen := lbEncode(this.willPropertiesLen) // 遗嘱属性长度
		copy(dst[total:], wpLen)
		total += len(wpLen)

		if this.willDelayInterval > 0 {
			dst[total] = DelayWills
			total++
			binary.BigEndian.PutUint32(dst[total:], this.willDelayInterval) // 遗嘱延时间隔
			total += 4
		}
		if this.payloadFormatIndicator > 0 {
			dst[total] = LoadFormatDescription
			total++
			dst[total] = this.payloadFormatIndicator // 遗嘱载荷指示
			total++
		}
		if this.willMsgExpiryInterval > 0 {
			dst[total] = MessageExpirationTime
			total++
			binary.BigEndian.PutUint32(dst[total:], this.willMsgExpiryInterval) // 遗嘱消息过期间隔
			total += 4
		}
		if len(this.contentType) > 0 {
			dst[total] = ContentType // 遗嘱内容类型
			total++
			n, err = writeLPBytes(dst[total:], this.contentType)
			total += n
			if err != nil {
				return total, err
			}
		}
		if len(this.responseTopic) > 0 {
			dst[total] = ResponseTopic // 遗嘱响应主题
			total++
			n, err = writeLPBytes(dst[total:], this.responseTopic)
			total += n
			if err != nil {
				return total, err
			}
		}
		if len(this.correlationData) > 0 {
			dst[total] = RelatedData // 对比数据
			total++
			n, err = writeLPBytes(dst[total:], this.correlationData)
			total += n
			if err != nil {
				return total, err
			}
		}
		for _, v1 := range this.willUserProperty {
			dst[total] = UserProperty // 遗嘱用户属性
			total++
			n, err = writeLPBytes(dst[total:], v1)
			total += n
			if err != nil {
				return total, err
			}
		}

		n, err = writeLPBytes(dst[total:], this.willTopic) // 遗嘱主题
		total += n
		if err != nil {
			return total, err
		}

		n, err = writeLPBytes(dst[total:], this.willMessage) // 遗嘱载荷
		total += n
		if err != nil {
			return total, err
		}
	}

	// According to the 3.1 spec, it's possible that the usernameFlag is set,
	// but the username string is missing.
	if this.UsernameFlag() && len(this.username) > 0 {
		n, err = writeLPBytes(dst[total:], this.username) // 用户名
		total += n
		if err != nil {
			return total, err
		}
	}

	// According to the 3.1 spec, it's possible that the passwordFlag is set,
	// but the password string is missing.
	if this.PasswordFlag() && len(this.password) > 0 {
		n, err = writeLPBytes(dst[total:], this.password) // 密码
		total += n
		if err != nil {
			return total, err
		}
	}

	return total, nil
}

func (this *ConnectMessage) decodeMessage(src []byte) (int, error) {
	var err error
	n, total := 0, 0

	// 协议名
	this.protoName, n, err = readLPBytes(src[total:])
	total += n
	if err != nil {
		return total, err
	} // 如果服务端不愿意接受 CONNECT 但希望表明其 MQTT 服务端身份，可以发送包含原因码为 0x84（不支持的协议版本）的 CONNACK 报文，然后必须关闭网络连接
	// 协议级别，版本号, 1 byte
	this.version = src[total]
	total++

	if verstr, ok := SupportedVersions[this.version]; !ok { // todo 发送原因码0x84（不支持的协议版本）的CONNACK报文，然后必须关闭网络连接
		return total, UnSupportedProtocolVersion // 如果协议版本不是 5 且服务端不愿意接受此 CONNECT 报文，可以发送包含原因码 0x84（不支持的协议版本）的CONNACK 报文，然后必须关闭网络连接
	} else if verstr != string(this.protoName) {
		return total, ProtocolError
	}

	// 连接标志
	// 7: username flag
	// 6: password flag
	// 5: will retain
	// 4-3: will QoS
	// 2: will flag
	// 1: clean session
	// 0: reserved
	this.connectFlags = src[total]
	total++

	// 服务端必须验证CONNECT报文的保留标志位（第 0 位）是否为 0 [MQTT-3.1.2-3]，如果不为0则此报文为无效报文
	if this.connectFlags&0x1 != 0 {
		return total, InvalidMessage // fmt.Errorf("connect/decodeMessage: Connect Flags reserved bit 0 is not 0")
	}

	if this.WillQos() > QosExactlyOnce { // 校验qos
		return total, InvalidMessage // fmt.Errorf("connect/decodeMessage: Invalid QoS level (%d) for %s message", this.WillQos(), this.Name())
	}
	// 遗嘱标志为0，will retain 和 will QoS都必须为0
	if !this.WillFlag() && (this.WillRetain() || this.WillQos() != QosAtMostOnce) {
		return total, InvalidMessage // fmt.Errorf("connect/decodeMessage: Protocol violation: If the Will Flag (%t) is set to 0 the Will QoS (%d) and Will Retain (%t) fields MUST be set to zero", this.WillFlag(), this.WillQos(), this.WillRetain())
	}

	// 用户名设置了，但是密码没有设置，也是无效报文
	// todo 相比MQTT v3.1.1，v5版本协议允许在没有用户名的情况下发送密码。这表明密码除了作为口令之外还可以有其他用途。
	if this.UsernameFlag() && !this.PasswordFlag() {
		return total, InvalidMessage // fmt.Errorf("connect/decodeMessage: Username flag is set but Password flag is not set")
	}

	if len(src[total:]) < 2 { // 判断是否还有超过2字节，要判断keepalive
		return 0, InvalidMessage // fmt.Errorf("connect/decodeMessage: Insufficient buffer size. Expecting %d, got %d.", 2, len(src[total:]))
	}
	// 双字节整数来表示以秒为单位的时间间隔
	// 时间为：18小时12分15秒
	// 如果保持连接的值非零，并且服务端在1.5倍的保持连接时间内没有收到客户端的控制报文，
	//     它必须断开客户端的网络连接，并判定网络连接已断开
	this.keepAlive = binary.BigEndian.Uint16(src[total:])
	total += 2

	// 属性

	this.propertiesLen, n, err = lbDecode(src[total:]) // 属性长度
	total += n
	if err != nil {
		return total, InvalidMessage
	}
	if len(src[total:]) < int(this.propertiesLen) {
		return total, InvalidMessage
	}

	if total < len(src) && src[total] == SessionExpirationInterval { //会话过期间隔（Session Expiry Interval）标识符。 四字节过期间隔，0xFFFFFFFF表示永不过期，0或者不设置，则为网络连接关闭时立即结束
		total++
		this.sessionExpiryInterval = binary.BigEndian.Uint32(src[total:])
		total += 4
		if total < len(src) && src[total] == SessionExpirationInterval {
			return total, ProtocolError
		}
	}

	if total < len(src) && src[total] == MaximumQuantityReceived { // 接收最大值（Receive Maximum）标识符。
		total++
		this.receiveMaximum = binary.BigEndian.Uint16(src[total:])
		total += 2
		if this.receiveMaximum == 0 || (total < len(src) && src[total] == MaximumQuantityReceived) {
			return total, ProtocolError
		}
	} else {
		this.receiveMaximum = 65535
	}

	if total < len(src) && src[total] == MaximumMessageLength { // 最大报文长度（Maximum Packet Size）标识符。
		total++
		this.maxPacketSize = binary.BigEndian.Uint32(src[total:])
		total += 4
		if this.maxPacketSize == 0 || (total < len(src) && src[total] == MaximumMessageLength) {
			return total, ProtocolError
		}
	} else {
		// TODO 如果没有 设置最大报文长度（Maximum Packet Size），则按照协议由固定报头中的剩余长度可编码最大值和协议 报头对数据包的大小做限制。
		// 服务端不能发送超过最大报文长度（Maximum Packet Size）的报文给客户端 [MQTT-3.1.2-24]。收到长度
		// 超过限制的报文将导致协议错误，客户端发送包含原因码 0x95（报文过大）的 DISCONNECT 报文给服务端
		// 当报文过大而不能发送时，服务端必须丢弃这些报文，然后当做应用消息发送已完成处理 [MQTT-3.1.2-25]。
		// 共享订阅的情况下，如果一条消息对于部分客户端来说太长而不能发送，服务端可以选择丢弃此消息或者把消息发送给剩余能够接收此消息的客户端
	}

	if total < len(src) && src[total] == MaximumLengthOfTopicAlias { // 主题别名最大值
		total++
		// 服务端在一个 PUBLISH 报文中发送的主题别名不能超过客户端设置的
		// 主题别名最大值（Topic Alias Maximum） [MQTT-3.1.2-26]。值为零表示本次连接客户端不接受任何主题
		// 别名（Topic Alias）。如果主题别名最大值（Topic Alias）没有设置，或者设置为零，则服务端不能向此客
		// 户端发送任何主题别名（Topic Alias）
		this.topicAliasMax = binary.BigEndian.Uint16(src[total:])
		total += 2
		if total < len(src) && src[total] == MaximumLengthOfTopicAlias {
			return total, ProtocolError
		}
	}

	if total < len(src) && src[total] == RequestResponseInformation { // 请求响应信息
		total++
		this.requestRespInfo = src[total]
		total++
		if (this.requestRespInfo != 0 && this.requestRespInfo != 1) ||
			(total < len(src) && src[total] == RequestResponseInformation) {
			return total, ProtocolError
		}
	}

	if total < len(src) && src[total] == RequestProblemInformation { // 请求问题信息
		total++
		// 客户端使用此值指示遇到错误时是否发送原因字符串（Reason String）或用户属性（User Properties）
		// 如果此值为 1，服务端可以在任何被允许的报文中返回原因字符串（Reason String）或用户属性（User Properties）
		this.requestProblemInfo = src[total]
		total++
		if (this.requestProblemInfo != 0 && this.requestProblemInfo != 1) ||
			(total < len(src) && src[total] == RequestProblemInformation) {
			return total, ProtocolError
		}
	} else {
		this.requestProblemInfo = 0x01
	}

	if total < len(src) && src[total] == UserProperty {
		total++
		this.userProperty = make([][]byte, 0)
		var up []byte
		up, n, err = readLPBytes(src[total:])
		total += n
		if err != nil {
			return total, err
		}
		this.userProperty = append(this.userProperty, up)
		for total < len(src) && src[total] == UserProperty {
			total++
			up, n, err = readLPBytes(src[total:])
			total += n
			if err != nil {
				return total, err
			}
			this.userProperty = append(this.userProperty, up)
		}
	}

	if total < len(src) && src[total] == AuthenticationMethod { // 认证方法
		// 如果客户端在 CONNECT 报文中设置了认证方法，则客户端在收到 CONNACK 报文之前不能发送除AUTH 或 DISCONNECT 之外的报文 [MQTT-3.1.2-30]。
		total++

		this.authMethod, n, err = readLPBytes(src[total:])
		total += n
		if err != nil {
			return total, err
		}

		if total < len(src) && src[total] == AuthenticationMethod {
			return total, ProtocolError
		}
	}

	if total < len(src) && src[total] == AuthenticationData { // 认证数据
		total++
		this.authData, n, err = readLPBytes(src[total:])
		total += n
		if err != nil {
			return total, err
		}
		if total < len(src) && src[total] == AuthenticationData {
			return total, ProtocolError
		}
	} else if len(this.authMethod) != 0 { // 有认证方法，却没有认证数据
		return total, ProtocolError
	}

	// 载荷
	// ==== 客户标识符 ====
	this.clientId, n, err = readLPBytes(src[total:]) // 客户标识符
	total += n
	if err != nil {
		return total, err
	}
	// If the Client supplies a zero-byte ClientId, the Client MUST also set CleanSession to 1
	if len(this.clientId) == 0 && !this.CleanSession() {
		return total, CustomerIdentifierInvalid
	}
	// The ClientId must contain only characters 0-9, a-z, and A-Z,-,_,/
	// We also support ClientId longer than 23 encoded bytes
	// We do not support ClientId outside of the above characters
	if len(this.clientId) > 0 && !this.validClientId(this.clientId) {
		return total, CustomerIdentifierInvalid
	}
	if len(this.clientId) == 0 {
		// 服务端可以允许客户端提供一个零字节的客户标识符 (ClientID)如果这样做了，服务端必须将这看作特殊
		// 情况并分配唯一的客户标识符给那个客户端 [MQTT-3.1.3-6]。
		// 然后它必须假设客户端提供了那个唯一的客户标识符，正常处理这个 CONNECT 报文 [MQTT-3.1.3-7]。
		this.clientId = defaultCId
	}
	// ==== 遗嘱属性 ====
	if this.WillFlag() { // 遗嘱属性
		// 遗嘱属性长度
		this.willPropertiesLen, n, err = lbDecode(src[total:])
		total += n
		if err != nil {
			return total, ProtocolError
		}
		if total < len(src) && src[total] == DelayWills { // 遗嘱延时间隔
			total++
			this.willDelayInterval = binary.BigEndian.Uint32(src[total:])
			total += 4
			if total < len(src) && src[total] == DelayWills {
				return total, ProtocolError
			}
		}
		if total < len(src) && src[total] == LoadFormatDescription { // 载荷格式指示
			total++
			this.payloadFormatIndicator = src[total]
			total++
			if this.payloadFormatIndicator != 0x00 && this.payloadFormatIndicator != 0x01 {
				return total, ProtocolError
			}
			if total < len(src) && src[total] == LoadFormatDescription {
				return total, ProtocolError
			}
		}
		if total < len(src) && src[total] == MessageExpirationTime { // 消息过期间隔
			total++
			this.willMsgExpiryInterval = binary.BigEndian.Uint32(src[total:])
			total += 4
			if total < len(src) && src[total] == MessageExpirationTime {
				return total, ProtocolError
			}
		}
		if total < len(src) && src[total] == ContentType { // 内容类型
			total++
			this.contentType, n, err = readLPBytes(src[total:])
			total += n
			if err != nil {
				return total, err
			}
			if total < len(src) && src[total] == ContentType {
				return total, ProtocolError
			}
		}
		if total < len(src) && src[total] == ResponseTopic { // 响应主题的存在将遗嘱消息（Will Message）标识为一个请求报文
			total++
			this.responseTopic, n, err = readLPBytes(src[total:])
			total += n
			if err != nil {
				return total, err
			}
			if total < len(src) && src[total] == ResponseTopic {
				return total, ProtocolError
			}
		}
		if total < len(src) && src[total] == RelatedData { // 对比数据 只对请求消息（Request Message）的发送端和响应消息（Response Message）的接收端有意义。
			total++
			this.correlationData, n, err = readLPBytes(src[total:])
			total += n
			if err != nil {
				return total, err
			}
			if total < len(src) && src[total] == RelatedData {
				return total, ProtocolError
			}
		}
		if total < len(src) && src[total] == UserProperty { // 用户属性
			total++
			this.willUserProperty = make([][]byte, 0)
			var tp []byte
			tp, n, err = readLPBytes(src[total:])
			total += n
			if err != nil {
				return total, err
			}
			this.willUserProperty = append(this.willUserProperty, tp)
			for total < len(src) && src[total] == UserProperty {
				total++
				tp, n, err = readLPBytes(src[total:])
				total += n
				if err != nil {
					return total, err
				}
				this.willUserProperty = append(this.willUserProperty, tp)
			}
		}

	}
	// ==== 遗嘱主题，遗嘱载荷 ====
	if this.WillFlag() {
		this.willTopic, n, err = readLPBytes(src[total:])
		total += n
		if err != nil {
			return total, err
		}

		this.willMessage, n, err = readLPBytes(src[total:])
		total += n
		if err != nil {
			return total, err
		}
	}

	// ==== 用户名 ====
	// According to the 3.1 spec, it's possible that the passwordFlag is set,
	// but the password string is missing.
	if this.UsernameFlag() && len(src[total:]) > 0 {
		this.username, n, err = readLPBytes(src[total:])
		total += n
		if err != nil {
			return total, err
		}
	}

	// ==== 密码 ====
	// According to the 3.1 spec, it's possible that the passwordFlag is set,
	// but the password string is missing.
	if this.PasswordFlag() && len(src[total:]) > 0 {
		this.password, n, err = readLPBytes(src[total:])
		total += n
		if err != nil {
			return total, err
		}
	}

	return total, nil
}

func (this *ConnectMessage) msglen() int {
	this.build()
	return int(this.remlen)
}

// validClientId checks the client ID, which is a slice of bytes, to see if it's valid.
// Client ID is valid if it meets the requirement from the MQTT spec:
// 		The Server MUST allow ClientIds which are between 1 and 23 UTF-8 encoded bytes in length,
//		and that contain only the characters
//
//		"0123456789abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ"
// 虽然协议写了不能超过23字节，和上面那些字符。
// 但实现还是可以不用完全有那些限制
func (this *ConnectMessage) validClientId(cid []byte) bool {
	// Fixed https://github.com/surgemq/surgemq/issues/4
	//if len(cid) > 23 {
	//	return false
	//}
	//if this.Version() == 0x05 {
	//	return true
	//}

	return clientIdRegexp.Match(cid)
}
