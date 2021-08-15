package message

import "errors"

// 变长字节整数解决方案

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

// 判断下一个标识符是哪个
func endIndexByTags(src []byte) int {
	for i := 0; i < len(src); i++ {
		switch src[i] {
		case UserProperty:

		}
	}
	return -1
}
