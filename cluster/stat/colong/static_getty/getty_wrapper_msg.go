package static_getty

import (
	"encoding/binary"
	"errors"
	"fmt"
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
)

// CMsgType 前4位用来表示类型，和mqtt消息类型一样处理
// 剩余4bit，|....|           3         |         2        |     1      |    0    |  为1时分别表示
//                |共享主题消息，必有tag|     有无msg体    |  有无tag   |   为0   |
//                                        status不需要msg
//                                     ping和pingresp也可以没有
type CMsgType byte

const (
	_                CMsgType = iota
	PubCMsg                   // 普通消息
	PubShareCMsg              // 共享主题消息
	PubShareRespCMsg          // 共享主题消息回应
	SubCMsg
	UnSubCMsg
	PingCMsg
	PingRespCMsg
	StatusCMsg   // 状态消息，可以通知其发慢点
	CloseSession // 通知集群删除某个session，断开某个client连接

	END
	Tag byte = 0x01 // CMsgType中的tag字段为1时，第二个字节必须是此标志，第0位为1表示后面还有tag，为0表示后面没有tag了，为msg了
)

type WrapCMsg interface {
	Type() CMsgType
	Tag() []string
	Share() bool
	Msg() messagev5.Message
	Status() map[string]string // 状态数据，自定义
	CloseSessions() []string   // 需要断开连接并删除的session，client ids

	SetShare(shareName string, msg messagev5.Message)
	AddTag(tag string)
	SetMsg(msg messagev5.Message)
	Len() int
}
type wrapCMsgImpl struct {
	cmsgtype byte
	tag      []string // 一个字节开头表示，和mqtt协议内协同的UTF-8字符串数据
	status   map[string]string
	msg      messagev5.Message // mqtt报文
}

func NewWrapCMsgImpl(msgType CMsgType) WrapCMsg {
	return &wrapCMsgImpl{
		cmsgtype: byte(msgType) << 4,
	}
}

// Status 状态数据在tag中采用 k v k v k v 排列
func (w *wrapCMsgImpl) Status() map[string]string {
	if w.Type() == StatusCMsg {
		if w.status == nil {
			w.status = make(map[string]string)
			for i := 0; i < len(w.tag)-1; i += 2 {
				w.status[w.tag[i]] = w.tag[i+1]
			}
		}
		return w.status
	}
	return nil
}

func (w *wrapCMsgImpl) CloseSessions() []string {
	if w.Type() == CloseSession {
		return w.tag
	}
	return nil
}

// IsShare 如果是共享消息，tag只能有共享组名称，且一个
func (w *wrapCMsgImpl) SetShare(shareName string, msg messagev5.Message) {
	w.tag = []string{shareName}
	w.cmsgtype |= 14 // tag位和msg位和share位都为1
	w.SetMsg(msg)
}
func (w *wrapCMsgImpl) AddTag(tag string) {
	w.tag = append(w.tag, tag)
	w.cmsgtype |= 2
}
func (w *wrapCMsgImpl) SetMsg(msg messagev5.Message) {
	w.msg = msg
	w.cmsgtype |= 4
}
func (w *wrapCMsgImpl) Len() int {
	total := 1
	for i := 0; i < len(w.tag); i++ {
		total += 3
		total += len(w.tag[i])
	}
	total += w.msg.Len()
	return total
}

// EncodeCMsg 将消息编码
func EncodeCMsg(msg WrapCMsg) ([]byte, error) {
	cmsg, ok := msg.(*wrapCMsgImpl)
	if !ok {
		return nil, errors.New("unSupport WrapCMsg, please use NewWrapCMsgImpl()")
	}
	b := make([]byte, cmsg.Len())
	total := 0
	b[total] = cmsg.cmsgtype
	total++
	for i := 0; i < len(cmsg.tag); i++ {
		if i < len(cmsg.tag)-1 {
			b[total] = 1
		} else {
			b[total] = 0
		}
		total++
		n, err := writeLPBytes(b[total:], []byte(cmsg.tag[i]))
		total += n
		if err != nil {
			return nil, err
		}
	}
	if cmsg.msg != nil {
		_, err := cmsg.msg.Encode(b[total:])
		if err != nil {
			return nil, err
		}
		//fmt.Println(n)
	}
	return b, nil
}

// DecodeCMsg 从消息体中解码包装的Msg，第二个返回值表示此处读取到的位置
func DecodeCMsg(b []byte) (WrapCMsg, int, error) {
	if len(b) <= 2 {
		return nil, 0, nil
	}
	var total int
	cmsgtype := CMsgType(b[0] >> 4)
	total++
	if cmsgtype <= 0 || cmsgtype >= END {
		return nil, total, errors.New("wrap msg type unSupport")
	}
	share := b[0]&8 == 8
	if share && b[0]&14 != 14 {
		return nil, total, errors.New("protocol error")
	}
	cmsg := &wrapCMsgImpl{cmsgtype: b[0]}
	if b[0]&2 == 2 { // 有tag
		for {
			cur := b[total]
			if cur == 1 || cur == 0 { // 这个也可以简化，将后面两字节长度字段，取1位来表示这个也可以，这里就没有去实现了
				total++
				tag, n, err := readLPBytes(b[total:])
				total += n
				if err != nil {
					return nil, total, err
				}
				cmsg.tag = append(cmsg.tag, string(tag))
				if cur == 0 {
					break
				}
			} else {
				return nil, total, errors.New("protocol error")
			}
		}
	}
	if b[0]&4 == 4 { // 有msg
		start := total
		dataLen := len(b[total:])
		if dataLen == 0 {
			return nil, 0, nil
		}
		cnt := 2
		for {
			// If we have read 5 bytes and still not done, then there's a problem.
			//如果我们已经读取了5个字节，但是仍然没有完成，那么就有一个问题。
			if cnt > 5 {
				// 剩余长度的第4个字节设置了延续位
				return nil, 0, nil
			}
			//如果没有返回足够的字节，则继续，直到有足够的字节。
			if len(b[total:]) < cnt {
				return nil, 0, nil
			}
			//如果获得了足够的字节，则检查最后一个字节，看看是否延续
			// 如果是，则增加cnt并继续窥视
			if b[cnt+total-1] >= 0x80 {
				cnt++
			} else {
				break
			}
		}

		tmpn := 0
		mtype := messagev5.MessageType(b[total] >> 4)
		tmpn++

		msglen, n, err := lbDecode(b[total+tmpn:])
		tmpn += n
		if err != nil {
			return nil, total + tmpn, err
		}

		//获取消息的剩余长度
		remlen := len(b[total+tmpn:])
		if remlen < int(msglen) {
			return nil, 0, nil
		}

		//消息的总长度
		//total2 := int(msglen) + n +1

		msg, err := mtype.New()
		if err != nil {
			return nil, total + tmpn, err
		}
		bnew := make([]byte, int(msglen)+n+1)
		copy(bnew, b[start:]) // 需要copy
		n, err = msg.Decode(bnew)
		total += n
		if err != nil {
			return nil, total, err
		}
		cmsg.msg = msg
	}
	return cmsg, total, nil
}

func (w *wrapCMsgImpl) Share() bool {
	return w.cmsgtype&8 == 8
}
func (w *wrapCMsgImpl) Type() CMsgType {
	return CMsgType(w.cmsgtype >> 4)
}

func (w *wrapCMsgImpl) Tag() []string {
	return w.tag
}

func (w *wrapCMsgImpl) Msg() messagev5.Message {
	return w.msg
}

// Read length prefixed bytes
//读取带前缀的字节长度，2 byte
func readLPBytes(buf []byte) ([]byte, int, error) {
	if len(buf) < 2 {
		return nil, 0, fmt.Errorf("utils/readLPBytes: Insufficient buffer size. Expecting %d, got %d.", 2, len(buf))
	}

	n, total := 0, 0
	// 两个字节长度
	n = int(binary.BigEndian.Uint16(buf))
	total += 2

	if len(buf) < n {
		return nil, total, fmt.Errorf("utils/readLPBytes: Insufficient buffer size. Expecting %d, got %d.", n, len(buf))
	}

	total += n

	return buf[2:total], total, nil
}

var maxLPString uint16 = 65535

// Write length prefixed bytes
func writeLPBytes(buf []byte, b []byte) (int, error) {
	total, n := 0, len(b)

	if n > int(maxLPString) {
		return 0, fmt.Errorf("utils/writeLPBytes: Length (%d) greater than %d bytes.", n, maxLPString)
	}

	if len(buf) < 2+n {
		return 0, fmt.Errorf("utils/writeLPBytes: Insufficient buffer size. Expecting %d, got %d.", 2+n, len(buf))
	}

	binary.BigEndian.PutUint16(buf, uint16(n))
	total += 2

	copy(buf[total:], b)
	total += n

	return total, nil
}
