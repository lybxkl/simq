package colong

import (
	"time"
)

import (
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"github.com/apache/dubbo-getty"
)

type MessageHandler2 struct {
	SessionOnOpen func(session getty.Session)
}

func (h *MessageHandler2) OnOpen(session getty.Session) error {
	log.Infof("OnOpen session{%s} open", session.Stat())
	if h.SessionOnOpen != nil {
		h.SessionOnOpen(session)
	}
	return nil
}

func (h *MessageHandler2) OnError(session getty.Session, err error) {
	log.Infof("OnError session{%s} got error{%v}, will be closed.", session.Stat(), err)
}

func (h *MessageHandler2) OnClose(session getty.Session) {
	log.Infof("OnClose session{%s} is closing......", session.Stat())
}

func (h *MessageHandler2) OnMessage(session getty.Session, pkg interface{}) {
	switch pkg := pkg.(type) {
	case *messagev5.ConnectMessage:
		log.Infof("OnMessage ConnectMessage: %s", pkg)
	case *messagev5.PingreqMessage:
		ping := messagev5.NewPingrespMessage()
		b := make([]byte, ping.Len())
		ping.Encode(b)
		_, err := session.WriteBytes(b)
		if err != nil {
			log.Infof("OnCorn session{%s} write bytes err: {%v}", session.Stat(), err)
		}
		log.Infof("OnMessage PingreqMessage: %s", pkg)
	default:
		log.Infof("OnMessage: %s", pkg)
	}
}

func (h *MessageHandler2) OnCron(session getty.Session) {
	active := session.GetActive()
	if CronPeriod < time.Since(active).Nanoseconds() {
		log.Infof("OnCorn session{%s} timeout{%s}", session.Stat(), time.Since(active).String())
		session.Close()
	}
}
