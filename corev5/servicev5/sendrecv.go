package servicev5

import (
	"encoding/binary"
	"fmt"
	"gitee.com/Ljolan/si-mqtt/corev5/logger"
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"io"
	"net"
	"time"
)

type netReader interface {
	io.Reader
	SetReadDeadline(t time.Time) error
}

type timeoutReader struct {
	d    time.Duration
	conn netReader
}

func (r timeoutReader) Read(b []byte) (int, error) {
	if err := r.conn.SetReadDeadline(time.Now().Add(r.d)); err != nil {
		return 0, err
	}
	return r.conn.Read(b)
}

// receiver() reads data from the network, and writes the data into the incoming buffer
func (this *service) receiver() {
	defer func() {
		// Let's recover from panic
		if r := recover(); r != nil {
			logger.Logger.Errorf("(%s) Recovering from panic: %v", this.cid(), r)
		}

		this.wgStopped.Done()

		logger.Logger.Debugf("(%s) Stopping receiver", this.cid())
	}()

	logger.Logger.Debugf("(%s) Starting receiver", this.cid())

	this.wgStarted.Done()

	switch conn := this.conn.(type) {
	// 普通tcp连接
	case net.Conn:
		// 如果保持连接的值非零，并且服务端在1.5倍的保持连接时间内没有收到客户端的控制报文，
		// 它必须断开客户端的网络连接，并判定网络连接已断开
		// 保持连接的实际值是由应用指定的，一般是几分钟。允许的最大值是18小时12分15秒(两个字节)
		// 保持连接（Keep Alive）值为零的结果是关闭保持连接（Keep Alive）机制。
		// 如果保持连接（Keep Alive）612 值为零，客户端不必按照任何特定的时间发送MQTT控制报文。
		keepAlive := time.Second * time.Duration(this.keepAlive)
		r := timeoutReader{
			d:    keepAlive + (keepAlive / 2),
			conn: conn,
		}

		for {
			_, err := this.in.ReadFrom(r)

			if err != nil {
				if err != io.EOF {
					//连接异常或者断线啥的
					//logger.Logger.Errorf("<<(%s)>> 连接异常关闭：%v", this.cid(), err.Error())
				}
				return
			}
		}
	//添加websocket，启动cl里有对websocket转tcp，这里就不用处理
	//case *websocket.Conn:
	//	glog.Errorf("(%s) Websocket: %v", this.cid(), ErrInvalidConnectionType)

	default:
		logger.Logger.Errorf("未知异常 (%s) %v", this.cid(), ErrInvalidConnectionType)
	}
}

// sender() writes data from the outgoing buffer to the network
func (this *service) sender() {
	defer func() {
		// Let's recover from panic
		if r := recover(); r != nil {
			logger.Logger.Errorf("(%s) Recovering from panic: %v", this.cid(), r)
		}

		this.wgStopped.Done()

		logger.Logger.Debugf("(%s) Stopping sender", this.cid())
	}()

	logger.Logger.Debugf("(%s) Starting sender", this.cid())

	this.wgStarted.Done()

	switch conn := this.conn.(type) {
	case net.Conn:
		for {
			_, err := this.out.WriteTo(conn)

			if err != nil {
				if err != io.EOF {
					logger.Logger.Errorf("(%s) error writing data: %v", this.cid(), err)
				}
				return
			}
		}

	//case *websocket.Conn:
	//	glog.Errorf("(%s) Websocket not supported", this.cid())

	default:
		logger.Logger.Infof("(%s) Invalid connection type", this.cid())
	}
}

// peekMessageSize() reads, but not commits, enough bytes to determine the size of
// the next messagev5 and returns the type and size.
// peekMessageSize()读取，但不提交，足够的字节来确定大小
//下一条消息，并返回类型和大小。
func (this *service) peekMessageSize() (messagev5.MessageType, int, error) {
	var (
		b   []byte
		err error
		cnt int = 2
	)

	if this.in == nil {
		err = ErrBufferNotReady
		return 0, 0, err
	}

	// Let's read enough bytes to get the messagev5 header (msg type, remaining length)
	//让我们读取足够的字节来获取消息头(msg类型，剩余长度)
	for {
		// If we have read 5 bytes and still not done, then there's a problem.
		//如果我们已经读取了5个字节，但是仍然没有完成，那么就有一个问题。
		if cnt > 5 {
			// 剩余长度的第4个字节设置了延续位
			return 0, 0, fmt.Errorf("sendrecv/peekMessageSize: 4th byte of remaining length has continuation bit set")
		}

		// Peek cnt bytes from the input buffer.
		//从输入缓冲区中读取cnt字节。
		b, err = this.in.ReadWait(cnt)
		if err != nil {
			return 0, 0, err
		}

		// If not enough bytes are returned, then continue until there's enough.
		//如果没有返回足够的字节，则继续，直到有足够的字节。
		if len(b) < cnt {
			continue
		}

		// If we got enough bytes, then check the last byte to see if the continuation
		// bit is set. If so, increment cnt and continue peeking
		//如果获得了足够的字节，则检查最后一个字节，看看是否延续
		// 如果是，则增加cnt并继续窥视
		if b[cnt-1] >= 0x80 {
			cnt++
		} else {
			break
		}
	}

	// Get the remaining length of the messagev5
	//获取消息的剩余长度
	remlen, m := binary.Uvarint(b[1:])

	// Total messagev5 length is remlen + 1 (msg type) + m (remlen bytes)
	//消息的总长度是remlen + 1 (msg类型)+ m (remlen字节)
	total := int(remlen) + 1 + m

	mtype := messagev5.MessageType(b[0] >> 4)

	return mtype, total, err
}

// peekMessage() reads a messagev5 from the buffer, but the bytes are NOT committed.
// This means the buffer still thinks the bytes are not read yet.
// peekMessage()从缓冲区读取消息，但是没有提交字节。
//这意味着缓冲区仍然认为字节还没有被读取。
func (this *service) peekMessage(mtype messagev5.MessageType, total int) (messagev5.Message, int, error) {
	var (
		b    []byte
		err  error
		i, n int
		msg  messagev5.Message
	)

	if this.in == nil {
		return nil, 0, ErrBufferNotReady
	}

	// Peek until we get total bytes
	//Peek，直到我们得到总字节数
	for i = 0; ; i++ {
		// Peek remlen bytes from the input buffer.
		//从输入缓冲区Peekremlen字节数。
		b, err = this.in.ReadWait(total)
		if err != nil && err != ErrBufferInsufficientData {
			return nil, 0, err
		}

		// If not enough bytes are returned, then continue until there's enough.
		//如果没有返回足够的字节，则继续，直到有足够的字节。
		if len(b) >= total {
			break
		}
	}

	msg, err = mtype.New()
	if err != nil {
		return nil, 0, err
	}

	n, err = msg.Decode(b)
	return msg, n, err
}

// readMessage() reads and copies a messagev5 from the buffer. The buffer bytes are
// committed as a result of the read.
func (this *service) readMessage(mtype messagev5.MessageType, total int) (messagev5.Message, int, error) {
	var (
		b   []byte
		err error
		n   int
		msg messagev5.Message
	)

	if this.in == nil {
		err = ErrBufferNotReady
		return nil, 0, err
	}

	if len(this.intmp) < total {
		this.intmp = make([]byte, total)
	}

	// Read until we get total bytes
	l := 0
	for l < total {
		n, err = this.in.Read(this.intmp[l:])
		l += n
		logger.Logger.Debugf("read %d bytes, total %d", n, l)
		if err != nil {
			return nil, 0, err
		}
	}

	b = this.intmp[:total]

	msg, err = mtype.New()
	if err != nil {
		return msg, 0, err
	}

	n, err = msg.Decode(b)
	return msg, n, err
}

// writeMessage() writes a messagev5 to the outgoing buffer
//writeMessage()将消息写入传出缓冲区
func (this *service) writeMessage(msg messagev5.Message) (int, error) {
	var (
		l    int = msg.Len()
		m, n int
		err  error
		buf  []byte
		wrap bool
	)

	if this.out == nil {
		return 0, ErrBufferNotReady
	}

	// This is to serialize writes to the underlying buffer. Multiple goroutines could
	// potentially get here because of calling Publish() or Subscribe() or other
	// functions that will send messages. For example, if a messagev5 is received in
	// another connetion, and the messagev5 needs to be published to this client, then
	// the Publish() function is called, and at the same time, another client could
	// do exactly the same thing.
	//
	// Not an ideal fix though. If possible we should remove mutex and be lockfree.
	// Mainly because when there's a large number of goroutines that want to publish
	// to this client, then they will all block. However, this will do for now.
	//
	//这是串行写入到底层缓冲区。 多了goroutine可能
	//可能是因为调用了Publish()或Subscribe()或其他方法而到达这里
	//发送消息的函数。 例如，如果接收到一条消息
	//另一个连接，消息需要被发布到这个客户端
	//调用Publish()函数，同时调用另一个客户机
	//做完全一样的事情。
	//
	//但这并不是一个理想的解决方案。
	//如果可能的话，我们应该移除互斥锁，并且是无锁的。
	//主要是因为有大量的goroutines想要发表文章
	//对于这个客户端，它们将全部阻塞。
	//但是，现在可以这样做。
	// FIXME: Try to find a better way than a mutex...if possible.
	this.wmu.Lock()
	defer this.wmu.Unlock()

	buf, wrap, err = this.out.WriteWait(l)
	if err != nil {
		return 0, err
	}

	if wrap {
		if len(this.outtmp) < l {
			this.outtmp = make([]byte, l)
		}

		n, err = msg.Encode(this.outtmp[0:])
		if err != nil {
			return 0, err
		}

		m, err = this.out.Write(this.outtmp[0:n])
		if err != nil {
			return m, err
		}
	} else {
		n, err = msg.Encode(buf[0:])
		if err != nil {
			return 0, err
		}

		m, err = this.out.WriteCommit(n)
		if err != nil {
			return 0, err
		}
	}

	this.outStat.increment(int64(m))

	return m, nil
}
