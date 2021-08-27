package colong

import (
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"github.com/apache/dubbo-getty"
)

var Sessions []getty.Session

func ClientRequest() {
	for _, session := range Sessions {
		ss := session
		go func() {
			echoTimes := 10
			for i := 0; i < echoTimes; i++ {
				msg := messagev5.NewConnectMessage()
				msg.SetWillQos(1)
				msg.SetVersion(5)
				msg.SetCleanSession(true)
				msg.SetClientId([]byte("surgemq"))
				msg.SetKeepAlive(30)
				msg.SetWillTopic([]byte("will"))
				msg.SetWillMessage([]byte("send me home"))
				msg.SetUsername([]byte("surgemq"))
				msg.SetPassword([]byte("verysecret"))
				msg.SetAuthMethod([]byte("default"))
				msg.SetAuthData([]byte("aaa"))
				b := make([]byte, msg.Len())
				msg.Encode(b)
				_, err := ss.WriteBytes(b)
				if err != nil {
					log.Infof("session.WritePkg(session{%s}, error{%v}", ss.Stat(), err)
					ss.Close()
				}
			}
			log.Infof("after loop %d times", echoTimes)
		}()
	}
}
