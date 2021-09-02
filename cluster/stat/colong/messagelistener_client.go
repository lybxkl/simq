package colong

import (
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"github.com/apache/dubbo-getty"
)

type messageHandler struct {
	SessionOnOpen func(name string, session getty.Session)
	inner
	curName string
}

func NewEventListener() getty.EventListener {
	return &messageHandler{inner: newInner()}
}
func SetCurName(listener getty.EventListener, curName string) {
	if listener, ok := listener.(*messageHandler); ok {
		listener.curName = curName
	}
}
func SetSessionOnOpen(listener getty.EventListener, fuc func(name string, session getty.Session)) {
	if listener, ok := listener.(*messageHandler); ok {
		listener.SessionOnOpen = fuc
	}
}
func (h *messageHandler) OnOpen(session getty.Session) error {
	log.Infof("OnOpen session{%s} open", session.Stat())
	msg := messagev5.NewConnectMessage()
	msg.SetVersion(5)
	msg.SetClientId([]byte(h.curName))
	msg.AddUserProperty([]byte("addr:127.0.0.1:8080"))
	b, _ := wrapperPub(msg)
	_, err := session.WriteBytes(b)
	if err != nil {
		log.Infof("session.WritePkg(session{%s}, error{%v}", session.Stat(), err)
		session.Close()
		return err
	}
	if h.SessionOnOpen != nil {
		h.SessionOnOpen(session.GetAttribute(Cname).(string), session)
	}
	return nil
}

func (h *messageHandler) OnError(session getty.Session, err error) {
	log.Infof("OnError session{%s} got error{%v}, will be closed.", session.Stat(), err)
}

func (h *messageHandler) OnClose(session getty.Session) {
	log.Infof("OnClose session{%s} is closing......", session.Stat())
}

func (h *messageHandler) OnMessage(session getty.Session, m interface{}) {
	pkg1, ok := m.(WrapCMsg)
	if !ok {
		return
	}
	pkg := pkg1.Msg()
	if pkg == nil {
		return
	}
	switch pkg := pkg.(type) {
	case *messagev5.PingrespMessage:
		// log.Infof("%s", pkg)
	case *messagev5.ConnackMessage:
		if pkg.ReasonCode() == messagev5.Success {
			// 此时才可以发送消息
			h.SetAuthOk(session, true)
		}
	}
}

var ping []byte

func init() {
	pi := messagev5.NewPingreqMessage()
	ping, _ = wrapperPub(pi)
}
func (h *messageHandler) OnCron(session getty.Session) {
	_, err := session.WriteBytes(ping)
	if err != nil {
		log.Infof("OnCorn session{%s} write bytes err: {%v}", session.Stat(), err)
	}
}
