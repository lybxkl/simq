package colong

import (
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"github.com/apache/dubbo-getty"
)

type MessageHandler struct {
	SessionOnOpen func(session getty.Session)
}

func (h *MessageHandler) OnOpen(session getty.Session) error {
	log.Infof("OnOpen session{%s} open", session.Stat())
	if h.SessionOnOpen != nil {
		h.SessionOnOpen(session)
	}
	return nil
}

func (h *MessageHandler) OnError(session getty.Session, err error) {
	log.Infof("OnError session{%s} got error{%v}, will be closed.", session.Stat(), err)
}

func (h *MessageHandler) OnClose(session getty.Session) {
	log.Infof("OnClose session{%s} is closing......", session.Stat())
}

func (h *MessageHandler) OnMessage(session getty.Session, pkg interface{}) {
	switch pkg.(type) {
	case *messagev5.PingrespMessage:
		log.Infof("%s", pkg)
	}
}

func (h *MessageHandler) OnCron(session getty.Session) {
	ping := messagev5.NewPingreqMessage()
	b := make([]byte, ping.Len())
	ping.Encode(b)
	_, err := session.WriteBytes(b)
	if err != nil {
		log.Infof("OnCorn session{%s} write bytes err: {%v}", session.Stat(), err)
	}
}
