package service

import (
	"encoding/binary"
	"errors"
	"fmt"
	messagev2 "gitee.com/Ljolan/si-mqtt/corev5/v2/message"
	getty "github.com/apache/dubbo-getty"
)

type packageHandler struct {
}

func (h *packageHandler) Read(ss getty.Session, data []byte) (interface{}, int, error) {
	var (
		cnt int = 2
	)
	// 让我们读取足够的字节来获取消息头(msg类型，剩余长度)
	for {
		// 如果我们已经读取了5个字节，但是仍然没有完成，那么就有一个问题。
		if cnt > 5 {
			// 剩余长度的第4个字节设置了延续位
			return nil, 0, fmt.Errorf("sendrecv/peekMessageSize: 4th byte of remaining length has continuation bit set")
		}

		// 如果没有返回足够的字节，则继续，直到有足够的字节。
		if len(data) < cnt {
			continue
		}
		// 如果获得了足够的字节，则检查最后一个字节，看看是否延续
		// 如果是，则增加cnt并继续窥视
		if data[cnt-1] >= 0x80 {
			cnt++
		} else {
			break
		}
	}

	//获取消息的剩余长度
	remlen, m := binary.Uvarint(data[1:])

	//消息的总长度是remlen + 1 (msg类型)+ m (remlen字节)
	total := int(remlen) + 1 + m
	if total > len(data) {
		return nil, 0, nil
	}

	mtype := messagev2.MessageType(data[0] >> 4)

	msg, err := mtype.New()
	if err != nil {
		return nil, 0, err
	}

	n, err := msg.Decode(data[:total])
	return msg, n, err
}

// 字节切片不会调用
func (h *packageHandler) Write(ss getty.Session, p interface{}) ([]byte, error) {
	var (
		msg messagev2.Message
		ok  bool
	)
	if msg, ok = p.(messagev2.Message); !ok {
		return nil, errors.New("消息类型错误")
	}

	data := make([]byte, msg.Len())
	_, err := msg.Encode(data)

	return data, err
}
