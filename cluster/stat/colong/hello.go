package colong

import (
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"github.com/apache/dubbo-getty"
	"time"
)

var Sessions []getty.Session

func ClientRequest() {
	for _, session := range Sessions {
		ss := session
		session := session
		go func() {
			echoTimes := 1
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
				msg.SetAuthData([]byte("1111"))
				msg.AddUserProperty([]byte("name:client1"))
				msg.AddUserProperty([]byte("addr:127.0.0.1:8080"))
				b := make([]byte, msg.Len())
				msg.Encode(b)
				_, err := ss.WriteBytes(b)
				if err != nil {
					log.Infof("session.WritePkg(session{%s}, error{%v}", ss.Stat(), err)
					ss.Close()
				}
			}
			log.Infof("after loop %d times", echoTimes)
			go func(session2 getty.Session) {
				time.Sleep(10)
				for i := 0; i < 100; i++ {
					if auth := session2.GetAttribute("auth"); auth != nil && auth.(bool) {
						pub := messagev5.NewPublishMessage()
						pub.SetQoS(0x01)
						pub.SetPacketId(123)
						pub.SetContentType([]byte("type"))
						pub.SetCorrelationData([]byte("pk"))
						pub.SetResponseTopic([]byte("/a/b/c"))
						pub.SetPayloadFormatIndicator(0x01)
						pub.SetPayload([]byte("载荷啦啦啦"))
						pub.AddUserPropertys([][]byte{[]byte("aaa:bb"), []byte("cc:ssd")})
						pub.SetMessageExpiryInterval(30)
						pub.SetTopic([]byte("p/p1/c"))
						pub.SetTopicAlias(100)
						b := make([]byte, pub.Len())
						_, err := pub.Encode(b)
						if err != nil {
							panic(err)
						}
						_, err = session2.WriteBytes(b)
						if err != nil {
							panic(err)
						}
					} else {
						//time.Sleep(1 * time.Second)
					}
				}
			}(session)
		}()
	}
}
