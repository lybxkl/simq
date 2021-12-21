package message

import (
	"bytes"
	"fmt"
)

// AuthMessage v5版本新增
type AuthMessage struct {
	header

	// 可变报头
	//如果原因码为 0x00（成功）并且没有属性字段，则可以省略原因码和属性长度。这种情况下，AUTH 报文 剩余长度为 0。

	reasonCode ReasonCode // 0x00,0x18,0x19
	//--- 属性
	propertiesLen uint32 // 属性长度
	authMethod    []byte
	authData      []byte
	reasonStr     []byte   // 如果加上原因字符串之后的 AUTH 报文长度超出了接收端所指定的最大报文长度，则发送端不能发送此属性
	userProperty  [][]byte // 如果加上用户属性之后的 AUTH 报文长度超出了接收端所指定的最大报文长度，则发送端不能发送此属性, 每对属性都是串行放置 [k1][v1][k2][v2]
	// AUTH 报文没有有效载荷
}

func (this *AuthMessage) ReasonCode() ReasonCode {
	return this.reasonCode
}

func (this *AuthMessage) SetReasonCode(reasonCode ReasonCode) {
	this.reasonCode = reasonCode
	this.dirty = true
}

func (this *AuthMessage) PropertiesLen() uint32 {
	return this.propertiesLen
}

func (this *AuthMessage) SetPropertiesLen(propertiesLen uint32) {
	this.propertiesLen = propertiesLen
	this.dirty = true
}

func (this *AuthMessage) AuthMethod() []byte {
	return this.authMethod
}

func (this *AuthMessage) SetAuthMethod(authMethod []byte) {
	this.authMethod = authMethod
	this.dirty = true
}

func (this *AuthMessage) AuthData() []byte {
	return this.authData
}

func (this *AuthMessage) SetAuthData(authData []byte) {
	this.authData = authData
	this.dirty = true
}

func (this *AuthMessage) ReasonStr() []byte {
	return this.reasonStr
}

func (this *AuthMessage) SetReasonStr(reasonStr []byte) {
	this.reasonStr = reasonStr
	this.dirty = true
}

func (this *AuthMessage) UserProperty() [][]byte {
	return this.userProperty
}

func (this *AuthMessage) AddUserPropertys(userProperty [][]byte) {
	this.userProperty = append(this.userProperty, userProperty...)
	this.dirty = true
}
func (this *AuthMessage) AddUserProperty(userProperty []byte) {
	this.userProperty = append(this.userProperty, userProperty)
	this.dirty = true
}

var _ Message = (*AuthMessage)(nil)

// NewAuthMessage creates a new AUTH message.
func NewAuthMessage() *AuthMessage {
	msg := &AuthMessage{}
	msg.SetType(AUTH)

	return msg
}

func (this AuthMessage) String() string {
	return fmt.Sprintf("%s, ReasonCode=%b, PropertiesLen=%d, AuthMethod=%s, AuthData=%q, ReasonStr=%s, UserProperty=%s",
		this.header,
		this.ReasonCode(),
		this.PropertiesLen(),
		this.AuthMethod(),
		this.AuthData(),
		this.ReasonStr(),
		this.UserProperty(),
	)
}

func (this *AuthMessage) Len() int {
	if !this.dirty {
		return len(this.dbuf)
	}

	ml := this.msglen()

	if err := this.SetRemainingLength(ml); err != nil {
		return 0
	}

	return this.header.msglen() + int(ml)
}

func (this *AuthMessage) Decode(src []byte) (int, error) {
	total := 0

	n, err := this.header.decode(src[total:])
	total += n
	if err != nil {
		return total, err
	}
	if int(this.header.remlen) > len(src[total:]) {
		return total, ProtocolError
	}

	if this.header.remlen == 0 {
		this.reasonCode = Success
		return total, nil
	}

	this.reasonCode = ReasonCode(src[total])
	total++
	if !ValidAuthReasonCode(this.reasonCode) {
		return total, ProtocolError
	}
	this.propertiesLen, n, err = lbDecode(src[total:])
	total += n
	if err != nil {
		return total, err
	}

	if total < len(src) && src[total] == AuthenticationMethod {
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
	if total < len(src) && src[total] == AuthenticationData {
		total++
		this.authData, n, err = readLPBytes(src[total:])
		total += n
		if err != nil {
			return total, err
		}
		if total < len(src) && src[total] == AuthenticationData {
			return total, ProtocolError
		}
	}
	if total < len(src) && src[total] == ReasonString {
		total++
		this.reasonStr, n, err = readLPBytes(src[total:])
		total += n
		if err != nil {
			return total, err
		}
		if total < len(src) && src[total] == ReasonString {
			return total, ProtocolError
		}
	}
	this.userProperty, n, err = decodeUserProperty(src[total:])
	total += n
	if err != nil {
		return total, err
	}

	this.dirty = false

	return total, nil
}

func (this *AuthMessage) Encode(dst []byte) (int, error) {
	if !this.dirty {
		if len(dst) < len(this.dbuf) {
			return 0, fmt.Errorf("auth/Encode: Insufficient buffer size. Expecting %d, got %d.", len(this.dbuf), len(dst))
		}

		return copy(dst, this.dbuf), nil
	}

	if this.Type() != AUTH {
		return 0, fmt.Errorf("auth/Encode: Invalid message type. Expecting %d, got %d", AUTH, this.Type())
	}

	ml := this.msglen()
	hl := this.header.msglen()

	ln := hl + int(ml)
	if len(dst) < ln {
		return 0, fmt.Errorf("auth/Encode: Insufficient buffer size. Expecting %d, got %d.", ln, len(dst))
	}

	if err := this.SetRemainingLength(ml); err != nil {
		return 0, err
	}

	total := 0

	n, err := this.header.encode(dst[total:])
	total += n
	if err != nil {
		return total, err
	}

	if this.reasonCode == Success && len(this.authMethod) == 0 && len(this.authData) == 0 && len(this.reasonStr) == 0 && len(this.userProperty) == 0 {
		return total, nil
	} else {
		dst[total] = byte(this.reasonCode)
		total++
	}
	n = copy(dst[total:], lbEncode(this.propertiesLen))
	total += n

	if len(this.authMethod) > 0 {
		dst[total] = AuthenticationMethod
		total++
		n, err = writeLPBytes(dst[total:], this.authMethod)
		total += n
		if err != nil {
			return total, err
		}
	}
	if len(this.authData) > 0 {
		dst[total] = AuthenticationData
		total++
		n, err = writeLPBytes(dst[total:], this.authData)
		total += n
		if err != nil {
			return total, err
		}
	}
	if len(this.reasonStr) > 0 {
		dst[total] = ReasonString
		total++
		n, err = writeLPBytes(dst[total:], this.reasonStr)
		total += n
		if err != nil {
			return total, err
		}
	}
	n, err = writeUserProperty(dst[total:], this.userProperty)
	total += n
	return total, err
}

func (this *AuthMessage) EncodeToBuf(dst *bytes.Buffer) (int, error) {
	if !this.dirty {
		return dst.Write(this.dbuf)
	}

	if this.Type() != AUTH {
		return 0, fmt.Errorf("auth/Encode: Invalid message type. Expecting %d, got %d", AUTH, this.Type())
	}

	ml := this.msglen()

	if err := this.SetRemainingLength(ml); err != nil {
		return 0, err
	}

	_, err := this.header.encodeToBuf(dst)
	if err != nil {
		return dst.Len(), err
	}

	if this.reasonCode == Success && len(this.authMethod) == 0 && len(this.authData) == 0 && len(this.reasonStr) == 0 && len(this.userProperty) == 0 {
		return dst.Len(), nil
	} else {
		dst.WriteByte(byte(this.reasonCode))
	}
	dst.Write(lbEncode(this.propertiesLen))

	if len(this.authMethod) > 0 {
		dst.WriteByte(AuthenticationMethod)
		_, err = writeToBufLPBytes(dst, this.authMethod)
		if err != nil {
			return dst.Len(), err
		}
	}
	if len(this.authData) > 0 {
		dst.WriteByte(AuthenticationData)
		_, err = writeToBufLPBytes(dst, this.authData)
		if err != nil {
			return dst.Len(), err
		}
	}
	if len(this.reasonStr) > 0 {
		dst.WriteByte(ReasonString)
		_, err = writeToBufLPBytes(dst, this.reasonStr)
		if err != nil {
			return dst.Len(), err
		}
	}
	return writeUserPropertyByBuf(dst, this.userProperty)
}

func (this *AuthMessage) build() {
	// == 可变报头 ==
	total := 1 // 认证原因码
	// 属性
	if len(this.authMethod) > 0 {
		total++
		total += 2
		total += len(this.authMethod)
	}
	if len(this.authData) > 0 {
		total++
		total += 2
		total += len(this.authData)
	}
	if len(this.reasonStr) > 0 { // todo 超过接收端指定的最大报文长度，不能发送
		total++
		total += 2
		total += len(this.reasonStr)
	}
	n := buildUserPropertyLen(this.userProperty)
	total += n

	this.propertiesLen = uint32(total - 1)

	if this.propertiesLen == 0 && this.reasonCode == Success {
		_ = this.SetRemainingLength(0)
		return
	}
	total += len(lbEncode(this.propertiesLen))
	_ = this.SetRemainingLength(uint32(total))
}

func (this *AuthMessage) msglen() uint32 {
	this.build()
	return this.remlen
}
