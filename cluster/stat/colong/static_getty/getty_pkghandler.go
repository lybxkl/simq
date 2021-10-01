package static_getty

import (
	"errors"
)

import (
	"github.com/apache/dubbo-getty"
)

type PackageHandler struct{}

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
func (h *PackageHandler) Read(ss getty.Session, data []byte) (interface{}, int, error) {
	msg, n, err := DecodeCMsg(data)
	if err != nil {
		return nil, 0, err
	}
	if msg == nil {
		return nil, n, nil
	}
	return msg, n, nil
}

// 字节切片不会调用
func (h *PackageHandler) Write(ss getty.Session, p interface{}) ([]byte, error) {
	pkg, ok := p.([]byte)
	if !ok {
		log.Infof("illegal pkg:%+v", p)
		return nil, errors.New("invalid package")
	}
	return pkg, nil
}
