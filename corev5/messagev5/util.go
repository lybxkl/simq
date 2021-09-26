package messagev5

import (
	"errors"
)

func CopyLen(data []byte, n int) []byte {
	if n <= 0 || len(data) == 0 {
		return make([]byte, 0)
	}
	if n > len(data) {
		n = len(data)
	}
	b := make([]byte, n)
	copy(b, data)
	return b
}

// 变长字节整数解决方案
// 可直接binary.PutUvarint()
func lbEncode(x uint32) []byte {
	encodedByte := x % 128
	b := make([]byte, 4)
	var i = 0
	x = x / 128
	if x > 0 {
		encodedByte = encodedByte | 128
		b[i] = byte(encodedByte)
		i++
	}
	for x > 0 {
		encodedByte = x % 128
		x = x / 128
		if x > 0 {
			encodedByte = encodedByte | 128
			b[i] = byte(encodedByte)
			i++
		}
	}
	b[i] = byte(encodedByte)
	return b[:i+1]
}

// 返回值，字节数，错误
// 可直接binary.Uvarint()
func lbDecode(b []byte) (uint32, int, error) {
	if len(b) == 0 {
		return 0, 0, nil
	}
	var (
		value, mu uint32 = 0, 1
		ec        byte
		i         = 0
	)
	ec, i = b[i], i+1
	value += uint32(ec&127) * mu
	mu *= 128
	for (ec & 128) != 0 {
		ec, i = b[i], i+1
		value += uint32(ec&127) * mu
		if mu > 128*128*128 {
			return 0, 0, errors.New("Malformed Variable Byte Integer")
		}
		mu *= 128
	}
	return value, i, nil
}

// ValidDisconnectReasonCode 验证disconnect原因码
func ValidDisconnectReasonCode(code ReasonCode) bool {
	switch code {
	case NormalDisconnected, DisconnectionIncludesWill, UnspecifiedError, ProtocolError,
		ImplementError, UnAuthorized, ServiceBusy, ServerBeingShutDown, KeepAliveTimeout, SessionTakeover,
		InvalidTopicFilter, InvalidTopicName, MaximumNumberReceivedExceeded, InvalidTopicAlias,
		MessageTooLong, TooManyMessages, BeyondQuota, ManagementBehavior, InvalidLoadFormat,
		UnsupportedRetention, UnsupportedQoSLevel, UseOtherServers, ServerHasMoved,
		UnsupportedSharedSubscriptions, ExceededConnectionRateLimit, MaximumConnectionTime,
		UnsupportedSubscriptionIdentifiers, UnsupportedWildcardSubscriptions:
		return true
	default:
		return false
	}
}

// ValidAuthReasonCode 验证auth原因码
func ValidAuthReasonCode(code ReasonCode) bool {
	switch code {
	case Success, ContinueAuthentication, ReAuthentication:
		return true
	default:
		return false
	}
}

// ValidUnSubAckReasonCode 验证UnSubAck原因码
func ValidUnSubAckReasonCode(code ReasonCode) bool {
	switch code {
	case Success, NoExistSubscription, UnspecifiedError, ImplementError,
		UnAuthorized, InvalidTopicFilter, PacketIdentifierIsOccupied:
		return true
	default:
		return false
	}
}

// ValidSubAckReasonCode 验证SubAck原因码
func ValidSubAckReasonCode(code ReasonCode) bool {
	switch code {
	case Success, Qos1, Qos2, UnspecifiedError, ImplementError,
		UnAuthorized, InvalidTopicFilter, PacketIdentifierIsOccupied, BeyondQuota,
		UnsupportedSharedSubscriptions, UnsupportedSubscriptionIdentifiers,
		UnsupportedWildcardSubscriptions:
		return true
	default:
		return false
	}
}

// ValidPubCompReasonCode 验证PubComp原因码
func ValidPubCompReasonCode(code ReasonCode) bool {
	switch code {
	case Success, PacketIdentifierInvalid:
		return true
	default:
		return false
	}
}

// ValidPubRelReasonCode 验证PubRel原因码
func ValidPubRelReasonCode(code ReasonCode) bool {
	switch code {
	case Success, PacketIdentifierInvalid:
		return true
	default:
		return false
	}
}

// ValidPubRecReasonCode 验证PubRec原因码
func ValidPubRecReasonCode(code ReasonCode) bool {
	switch code {
	case Success, NoMatchSubscription, UnspecifiedError, ImplementError, UnAuthorized,
		InvalidTopicName, PacketIdentifierIsOccupied, BeyondQuota, InvalidLoadFormat:
		return true
	default:
		return false
	}
}

// ValidPubAckReasonCode 验证PubAck原因码
func ValidPubAckReasonCode(code ReasonCode) bool {
	switch code {
	case Success, NoMatchSubscription, UnspecifiedError, ImplementError, UnAuthorized,
		InvalidTopicName, PacketIdentifierIsOccupied, BeyondQuota, InvalidLoadFormat:
		return true
	default:
		return false
	}
}

// ValidConnAckReasonCode 验证ConnAck原因码
func ValidConnAckReasonCode(code ReasonCode) bool {
	switch code {
	case Success, UnspecifiedError, InvalidMessage, ProtocolError, ImplementError,
		UnSupportedProtocolVersion, CustomerIdentifierInvalid, UserNameOrPasswordIsIncorrect,
		UnAuthorized, ServerUnavailable, ServiceBusy, Forbid, InvalidAuthenticationMethod,
		InvalidTopicName, MessageTooLong, BeyondQuota, InvalidLoadFormat, UnsupportedRetention,
		UnsupportedQoSLevel, UseOtherServers, ServerHasMoved, ExceededConnectionRateLimit:
		return true
	default:
		return false
	}
}
