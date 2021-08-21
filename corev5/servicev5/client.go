package servicev5

import (
	"fmt"
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"gitee.com/Ljolan/si-mqtt/corev5/sessionsv5"
	"gitee.com/Ljolan/si-mqtt/corev5/topicsv5"
	"net"
	"net/url"
	"sync/atomic"
	"time"
)

const (
	minKeepAlive = 30
)

// Client is a library implementation of the MQTT client that, as best it can, complies
// with the MQTT 3.1 and 3.1.1 specs.
// Client是MQTT客户机的一个库实现，它尽可能地进行了编译
//带有MQTT 3.1和3.1.1规范。
type Client struct {
	// The number of seconds to keep the connection live if there's no data.
	// If not set then default to 5 mins.
	KeepAlive int

	// The number of seconds to wait for the CONNACK messagev5 before disconnecting.
	// If not set then default to 2 seconds.
	ConnectTimeout int

	// The number of seconds to wait for any ACK messages before failing.
	// If not set then default to 20 seconds.
	AckTimeout int

	// The number of times to retry sending a packet if ACK is not received.
	// If no set then default to 3 retries.
	TimeoutRetries int

	svc *service
}

// Connect is for MQTT clients to open a connection to a remote server. It needs to
// know the URI, e.g., "tcp://127.0.0.1:1883", so it knows where to connect to. It also
// needs to be supplied with the MQTT CONNECT messagev5.
// Connect用于MQTT客户端打开到远程服务器的连接。它需要
//知道URI，例如，“tcp://127.0.0.1:1883”，因此它知道连接到哪里。它还
//需要与MQTT连接消息一起提供。
func (this *Client) Connect(uri string, msg *messagev5.ConnectMessage) (err error) {
	this.checkConfiguration()

	if msg == nil {
		return fmt.Errorf("msg is nil")
	}

	u, err := url.Parse(uri)
	if err != nil {
		return err
	}

	if u.Scheme != "tcp" {
		return ErrInvalidConnectionType
	}

	conn, err := net.Dial(u.Scheme, u.Host)
	if err != nil {
		return err
	}

	defer func() {
		if err != nil {
			conn.Close()
		}
	}()

	if msg.KeepAlive() < minKeepAlive {
		msg.SetKeepAlive(minKeepAlive)
	}

	if err = writeMessage(conn, msg); err != nil {
		return err
	}

	conn.SetReadDeadline(time.Now().Add(time.Second * time.Duration(this.ConnectTimeout)))

	resp, err := getConnackMessage(conn)
	if err != nil {
		return err
	}

	if resp.ReasonCode() != messagev5.Success {
		return resp.ReasonCode()
	}

	this.svc = &service{
		id:     atomic.AddUint64(&gsvcid, 1),
		client: true,
		conn:   conn,

		keepAlive:      int(msg.KeepAlive()),
		connectTimeout: this.ConnectTimeout,
		ackTimeout:     this.AckTimeout,
		timeoutRetries: this.TimeoutRetries,
	}

	err = this.getSession(this.svc, msg, resp)
	if err != nil {
		return err
	}

	p := topicsv5.NewMemProvider()
	topicsv5.Register(this.svc.sess.ID(), p)

	this.svc.topicsMgr, err = topicsv5.NewManager(this.svc.sess.ID())
	if err != nil {
		return err
	}

	if err := this.svc.start(); err != nil {
		this.svc.stop()
		return err
	}

	this.svc.inStat.increment(int64(msg.Len()))
	this.svc.outStat.increment(int64(resp.Len()))

	return nil
}

// Publish sends a single MQTT PUBLISH messagev5 to the server. On completion, the
// supplied OnCompleteFunc is called. For QOS 0 messages, onComplete is called
// immediately after the messagev5 is sent to the outgoing buffer. For QOS 1 messages,
// onComplete is called when PUBACK is received. For QOS 2 messages, onComplete is
// called after the PUBCOMP messagev5 is received.
// Publish向服务器发送单个MQTT发布消息。在完成, 调用
//provided OnCompleteFunc。对于QOS 0消息，将调用onComplete
//在消息发送到传出缓冲区之后立即执行。对于QOS 1消息，
//当收到PUBACK时调用onComplete。对于QOS 2消息，onComplete是
//在收到PUBCOMP消息后调用。
func (this *Client) Publish(msg *messagev5.PublishMessage, onComplete OnCompleteFunc) error {
	return this.svc.publish(msg, onComplete)
}

// Subscribe sends a single SUBSCRIBE messagev5 to the server. The SUBSCRIBE messagev5
// can contain multiple topicsv5 that the client wants to subscribe to. On completion,
// which is when the client receives a SUBACK messsage back from the server, the
// supplied onComplete funciton is called.
//
// When messages are sent to the client from the server that matches the topicsv5 the
// client subscribed to, the onPublish function is called to handle those messages.
// So in effect, the client can supply different onPublish functions for different
// topicsv5.
// Subscribe向服务器发送一条订阅消息。
//订阅消息
//可以包含客户端希望订阅的多个主题。
//收购完成后,
//当客户端收到来自服务器的一个SUBACK消息时
//提供一个完整的功能被调用。
//
//当消息从与主题匹配的服务器发送到客户端时
//客户端订阅后，调用onPublish函数来处理这些消息。
//因此，实际上，客户端可以为不同的onPublish函数提供不同的onPublish函数
//主题。
func (this *Client) Subscribe(msg *messagev5.SubscribeMessage, onComplete OnCompleteFunc, onPublish OnPublishFunc) error {
	return this.svc.subscribe(msg, onComplete, onPublish)
}

// Unsubscribe sends a single UNSUBSCRIBE messagev5 to the server. The UNSUBSCRIBE
// messagev5 can contain multiple topicsv5 that the client wants to unsubscribe. On
// completion, which is when the client receives a UNSUBACK messagev5 from the server,
// the supplied onComplete function is called. The client will no longer handle
// messages from the server for those unsubscribed topicsv5.
//取消订阅向服务器发送一条取消订阅消息。的退订
//消息可以包含多个客户端想要取消订阅的主题。在
//完成，当客户端收到来自服务器的UNSUBACK消息时，
//调用所提供的onComplete函数。客户端将不再处理
//来自服务器的未订阅主题的消息。
func (this *Client) Unsubscribe(msg *messagev5.UnsubscribeMessage, onComplete OnCompleteFunc) error {
	return this.svc.unsubscribe(msg, onComplete)
}

// Ping sends a single PINGREQ messagev5 to the server. PINGREQ/PINGRESP messages are
// mainly used by the client to keep a heartbeat to the server so the connection won't
// be dropped.
// Ping向服务器发送一条单独的PINGREQ消息。PINGREQ / PINGRESP消息
//客户端主要用来保持心跳到服务器，这样连接就不会dropped
func (this *Client) Ping(onComplete OnCompleteFunc) error {
	return this.svc.ping(onComplete)
}

// Disconnect sends a single DISCONNECT messagev5 to the server. The client immediately
// terminates after the sending of the DISCONNECT messagev5.
//向服务器发送一条断开连接的消息。客户端立即
//发送断开连接的消息后终止。
func (this *Client) Disconnect() {
	//msg := messagev5.NewDisconnectMessage()
	this.svc.stop()
}

func (this *Client) getSession(svc *service, req *messagev5.ConnectMessage, resp *messagev5.ConnackMessage) error {
	//id := string(req.ClientId())
	svc.sess = &sessionsv5.Session{}
	return svc.sess.Init(req)
}

func (this *Client) checkConfiguration() {
	if this.KeepAlive == 0 {
		this.KeepAlive = 30
	}

	if this.ConnectTimeout == 0 {
		this.ConnectTimeout = 20
	}

	if this.AckTimeout == 0 {
		this.AckTimeout = 15
	}

	if this.TimeoutRetries == 0 {
		this.TimeoutRetries = 2
	}
}
