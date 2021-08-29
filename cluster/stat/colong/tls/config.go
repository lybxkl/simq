package tls

import (
	"crypto/tls"
	"gitee.com/Ljolan/si-mqtt/cluster/stat/colong"
	"time"
)

import (
	"github.com/apache/dubbo-getty"
)

var (
	pkgHandler = &colong.PackageHandler{}
	// EventListener register event callback
	EventListener       = colong.NewEventListener()
	EventServerListener = colong.NewServerEventListener()
)

func InitialSession(session getty.Session) (err error) {
	_, ok := session.Conn().(*tls.Conn)
	if ok {
		session.SetName("hello")
		session.SetMaxMsgLen(128 * 1024) // max message package length is 128k
		session.SetReadTimeout(time.Second)
		session.SetWriteTimeout(5 * time.Second)
		session.SetCronPeriod(int(colong.CronPeriod / 1e6))
		session.SetWaitTime(time.Second)

		session.SetPkgHandler(pkgHandler)
		session.SetEventListener(EventListener)
	}
	return nil
}
func InitialSessionServer(session getty.Session) (err error) {
	_, ok := session.Conn().(*tls.Conn)
	if ok {
		session.SetName("hello")
		session.SetMaxMsgLen(128 * 1024) // max message package length is 128k
		session.SetReadTimeout(time.Second)
		session.SetWriteTimeout(5 * time.Second)
		session.SetCronPeriod(int(colong.CronPeriod / 1e6))
		session.SetWaitTime(time.Second)

		session.SetPkgHandler(pkgHandler)
		session.SetEventListener(EventServerListener)
	}
	return nil
}
