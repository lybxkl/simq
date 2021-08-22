package messagev5

import (
	"encoding/binary"
	"fmt"
)

// The DISCONNECT Packet is the final Control Packet sent from the Client to the Server.
// It indicates that the Client is disconnecting cleanly.
type DisconnectMessage struct {
	header
	reasonCode ReasonCode
	//  属性
	propertyLen uint32 // 如果剩余长度小于4字节，则没有属性长度
	// 会话过期间隔不能由服务端的DISCONNECT报文发送
	sessionExpiryInterval uint32   // 会话过期间隔 如果没有设置会话过期间隔，则使用CONNECT报文中的会话过期间隔
	reasonStr             []byte   // 长度超出了接收端指定的最大报文长度，则发送端不能发送此属性
	serverReference       []byte   // 客户端可以使用它来识别其他要使用的服务端
	userProperty          [][]byte // 用户属性如果加上之后的报文长度超出了接收端指定的最大报文长度（Maximum Packet Size），则发送端不能发送此用户属性
}

func (d *DisconnectMessage) String() string {
	return fmt.Sprintf("header: %v, reasonCode=%s, propertyLen=%v, sessionExpiryInterval=%v, reasonStr=%v, serverReference=%s, userProperty=%s",
		d.header, d.reasonCode, d.propertyLen, d.sessionExpiryInterval, d.reasonStr, d.serverReference, d.userProperty)
}

var _ Message = (*DisconnectMessage)(nil)

// NewDisconnectMessage creates a new DISCONNECT message.
func NewDisconnectMessage() *DisconnectMessage {
	msg := &DisconnectMessage{}
	msg.SetType(DISCONNECT)

	return msg
}

func (this *DisconnectMessage) Decode(src []byte) (int, error) {
	total, err := this.header.decode(src)
	if err != nil {
		return total, err
	}
	if this.remlen == 0 {
		this.reasonCode = Success
		return total, nil
	}
	this.reasonCode = ReasonCode(src[total])
	total++

	if !ValidDisconnectReasonCode(this.reasonCode) {
		return total, ProtocolError
	}

	if this.remlen < 2 {
		this.propertyLen = 0
		return total, nil
	}
	var n int
	this.propertyLen, n, err = lbDecode(src[total:])
	total += n
	if err != nil {
		return total, err
	}

	if total < len(src) && src[total] == SessionExpirationInterval {
		total++
		this.sessionExpiryInterval = binary.BigEndian.Uint32(src)
		total += 4
		if this.sessionExpiryInterval == 0 {
			return total, ProtocolError
		}
		if total < len(src) && src[total] == SessionExpirationInterval {
			return total, err
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
			return total, err
		}
	}
	if total < len(src) && src[total] == UserProperty {
		total++
		var up []byte
		up, n, err = readLPBytes(src[total:])
		total += n
		if err != nil {
			return total, err
		}
		this.AddUserProperty(up)
		for total < len(src) && src[total] == UserProperty {
			up, n, err = readLPBytes(src[total:])
			total += n
			if err != nil {
				return total, err
			}
			this.AddUserProperty(up)
		}
	}
	if total < len(src) && src[total] == ServerReference {
		total++
		this.serverReference, n, err = readLPBytes(src[total:])
		total += n
		if err != nil {
			return total, err
		}
		if total < len(src) && src[total] == ServerReference {
			return total, err
		}
	}
	return total, nil
}

func (this *DisconnectMessage) Encode(dst []byte) (int, error) {
	if !this.dirty {
		if len(dst) < len(this.dbuf) {
			return 0, fmt.Errorf("disconnect/Encode: Insufficient buffer size. Expecting %d, got %d.", len(this.dbuf), len(dst))
		}

		return copy(dst, this.dbuf), nil
	}
	ml := this.msglen()
	hl := this.header.msglen()

	if len(dst) < hl+ml {
		return 0, fmt.Errorf("auth/Encode: Insufficient buffer size. Expecting %d, got %d.", hl+ml, len(dst))
	}

	if err := this.SetRemainingLength(int32(ml)); err != nil {
		return 0, err
	}

	total, err := this.header.encode(dst)
	if err != nil {
		return total, err
	}
	if this.reasonCode == Success && this.remlen == 0 {
		return total, nil
	}
	dst[total] = this.reasonCode.Value()
	total++

	n := copy(dst[total:], lbEncode(this.propertyLen))
	total += n

	if this.sessionExpiryInterval > 0 {
		dst[total] = SessionExpirationInterval
		total++
		binary.BigEndian.PutUint32(dst[total:], this.sessionExpiryInterval)
		total += 4
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
	for i := 0; i < len(this.userProperty); i++ {
		dst[total] = UserProperty
		total++
		n, err = writeLPBytes(dst[total:], this.userProperty[i])
		total += n
		if err != nil {
			return total, err
		}
	}
	if len(this.serverReference) > 0 {
		dst[total] = ServerReference
		total++
		n, err = writeLPBytes(dst[total:], this.serverReference)
		total += n
		if err != nil {
			return total, err
		}
	}
	return total, nil
}
func (this *DisconnectMessage) build() {
	total := 0
	if this.sessionExpiryInterval > 0 {
		total += 5
	}
	if len(this.reasonStr) > 0 { // todo 超了就不发
		total++
		total += 2
		total += len(this.reasonStr)
	}
	for i := 0; i < len(this.userProperty); i++ { // todo 超了就不发
		total++
		total += 2
		total += len(this.userProperty[i])
	}
	if len(this.serverReference) > 0 {
		total++
		total += 2
		total += len(this.serverReference)
	}
	this.propertyLen = uint32(total)
	if this.reasonCode == Success && this.propertyLen == 0 {
		_ = this.SetRemainingLength(0)
		return
	}
	// 加 1 是断开原因码
	_ = this.SetRemainingLength(int32(1 + int(this.propertyLen) + len(lbEncode(this.propertyLen))))
}
func (this *DisconnectMessage) msglen() int {
	this.build()
	return int(this.remlen)
}
func (this *DisconnectMessage) Len() int {
	if !this.dirty {
		return len(this.dbuf)
	}

	ml := this.msglen()

	if err := this.SetRemainingLength(int32(ml)); err != nil {
		return 0
	}

	return this.header.msglen() + ml
}
func (this *DisconnectMessage) ReasonCode() ReasonCode {
	return this.reasonCode
}

func (this *DisconnectMessage) SetReasonCode(reasonCode ReasonCode) {
	this.reasonCode = reasonCode
	this.dirty = true
}

func (this *DisconnectMessage) PropertyLen() uint32 {
	return this.propertyLen
}

func (this *DisconnectMessage) SetPropertyLen(propertyLen uint32) {
	this.propertyLen = propertyLen
	this.dirty = true
}

func (this *DisconnectMessage) SessionExpiryInterval() uint32 {
	return this.sessionExpiryInterval
}

func (this *DisconnectMessage) SetSessionExpiryInterval(sessionExpiryInterval uint32) {
	this.sessionExpiryInterval = sessionExpiryInterval
	this.dirty = true
}

func (this *DisconnectMessage) ReasonStr() []byte {
	return this.reasonStr
}

func (this *DisconnectMessage) SetReasonStr(reasonStr []byte) {
	this.reasonStr = reasonStr
	this.dirty = true
}

func (this *DisconnectMessage) ServerReference() []byte {
	return this.serverReference
}

func (this *DisconnectMessage) SetServerReference(serverReference []byte) {
	this.serverReference = serverReference
	this.dirty = true
}

func (this *DisconnectMessage) UserProperty() [][]byte {
	return this.userProperty
}

func (this *DisconnectMessage) AddUserPropertys(userProperty [][]byte) {
	this.userProperty = append(this.userProperty, userProperty...)
	this.dirty = true
}
func (this *DisconnectMessage) AddUserProperty(userProperty []byte) {
	this.userProperty = append(this.userProperty, userProperty)
	this.dirty = true
}
