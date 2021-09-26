package colong

import (
	"fmt"
	"gitee.com/Ljolan/si-mqtt/cluster"
	"net"
	"time"
)

import (
	"github.com/apache/dubbo-getty"
)

var (
	pkgHandler = &PackageHandler{}
	// EventListener register event callback
	EventListener       = NewEventListener()
	EventServerListener = NewServerEventListener()
)

// InitialSession name是连接的服务端的名称
func InitialSession(name string, session getty.Session) (err error) {
	// session.SetCompressType(getty.CompressZip)

	tcpConn, ok := session.Conn().(*net.TCPConn)
	if !ok {
		panic(fmt.Sprintf("newSession: %s, session.conn{%#v} is not tcp connection", session.Stat(), session.Conn()))
	}

	if err = tcpConn.SetNoDelay(true); err != nil {
		return err
	}
	if err = tcpConn.SetKeepAlive(true); err != nil {
		return err
	}
	if err = tcpConn.SetKeepAlivePeriod(10 * time.Second); err != nil {
		return err
	}
	if err = tcpConn.SetReadBuffer(262144); err != nil {
		return err
	}
	if err = tcpConn.SetWriteBuffer(524288); err != nil {
		return err
	}

	session.SetName("client_" + name)
	session.SetMaxMsgLen(128 * 1024) // max message package length is 128k
	session.SetReadTimeout(time.Second)
	session.SetWriteTimeout(5 * time.Second)
	session.SetCronPeriod(3000)
	session.SetWaitTime(time.Second)
	session.SetAttribute(Cname, name)

	session.SetPkgHandler(pkgHandler)
	session.SetEventListener(EventListener)
	return nil
}

// InitialSessionServer 这里的name是本服务的名称，没啥用
func InitialSessionServer(name string, session getty.Session, clusterInToPub ClusterInToPub,
	clusterInToPubShare ClusterInToPubShare, clusterInToPubSys ClusterInToPubSys,
	shareTopicMapNode cluster.ShareTopicMapNode) (err error) {

	// session.SetCompressType(getty.CompressZip)

	tcpConn, ok := session.Conn().(*net.TCPConn)
	if !ok {
		panic(fmt.Sprintf("newSession: %s, session.conn{%#v} is not tcp connection", session.Stat(), session.Conn()))
	}

	if err = tcpConn.SetNoDelay(true); err != nil {
		return err
	}
	if err = tcpConn.SetKeepAlive(true); err != nil {
		return err
	}
	if err = tcpConn.SetKeepAlivePeriod(10 * time.Second); err != nil {
		return err
	}
	if err = tcpConn.SetReadBuffer(262144); err != nil {
		return err
	}
	if err = tcpConn.SetWriteBuffer(524288); err != nil {
		return err
	}

	session.SetName("_server_" + name)
	session.SetMaxMsgLen(128 * 1024) // max message package length is 128k
	session.SetReadTimeout(time.Second)
	session.SetWriteTimeout(5 * time.Second)
	session.SetCronPeriod(int(CronPeriod / 1e6))
	session.SetWaitTime(time.Second)

	session.SetPkgHandler(pkgHandler)
	SetPubFunc(EventServerListener, clusterInToPub, clusterInToPubShare,
		clusterInToPubSys, shareTopicMapNode)
	session.SetEventListener(EventServerListener)
	return nil
}
