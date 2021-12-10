package service

import (
	"fmt"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/auth/authplus"
	messagev52 "gitee.com/Ljolan/si-mqtt/corev5/v2/message"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/topics"
	logger2 "gitee.com/Ljolan/si-mqtt/logger"
	"github.com/stretchr/testify/require"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

var once sync.Once

// 设置环境变量 "SI_CFG_PATH" = "F:\\Go_pro\\src\\si-mqtt\\config"
func TestExampleClient(t *testing.T) {
	clientId := "surgemq"

	once.Do(func() {
		logger2.LogInit("info") // 日志必须提前初始化
	})

	vp := authplus.NewDefaultAuth()
	var pkid uint32 = 0

	c := &Client{}
	c.AuthPlus = vp
	// Creates a new MQTT CONNECT messagev5 and sets the proper parameters
	msg := messagev52.NewConnectMessage()
	msg.SetWillQos(1)
	msg.SetVersion(5)
	msg.SetCleanSession(true)
	msg.SetClientId([]byte(clientId))
	msg.SetKeepAlive(30)
	msg.SetWillTopic([]byte("will"))
	msg.SetWillMessage([]byte("send me home"))
	msg.SetUsername([]byte("surgemq"))
	msg.SetPassword([]byte("verysecret"))
	msg.SetAuthMethod([]byte("default"))
	msg.SetAuthData([]byte("aaa"))
	msg.SetTopicAliasMax(1000)
	// Connects to the remote server at 127.0.0.1 port 1883
	err := c.Connect("tcp://127.0.0.1:1883", msg)
	require.NoError(t, err)
	time.Sleep(2 * time.Second)

	// Creates a new PUBLISH messagev5 with the appropriate contents for publishing

	submsg := messagev52.NewSubscribeMessage()
	submsg.AddTopic([]byte("abc/#"), 1)
	//submsg.SetTopicNoLocal([]byte("abc"), true)
	fmt.Println("====== >>> Subscribe")
	err = c.Subscribe(submsg, func(msg, ack messagev52.Message, err error) error {
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println("\n====== >>> Subscribe msg handle")
			fmt.Println(ack)
		}
		return nil
	}, func(msg *messagev52.PublishMessage, sub topics.Sub, sender string, shareMsg bool) error {
		fmt.Println("===<<<>>>", msg.String())
		if msg.SubscriptionIdentifier() > 0 {
			// ...
		}
		if len(msg.ResponseTopic()) > 0 {
			// ... 发送到响应主题去
		}
		return nil
	})
	require.NoError(t, err)
	time.Sleep(5 * time.Second)

	// Publishes to the server by sending the messagev5
	fmt.Println("====== >>> Publish")
	for i := 0; i < 100; i++ {
		pubmsg := messagev52.NewPublishMessage()
		pubmsg.SetTopic([]byte("abc"))
		pubmsg.SetPayload([]byte("1234567890"))
		pubmsg.SetQoS(1)
		//pubmsg.SetTopicAlias(10)

		if pubmsg.QoS() > 0 {
			pubmsg.SetPacketId(uint16(atomic.AddUint32(&pkid, 1)))
		}
		err = c.Publish(pubmsg, func(msg, ack messagev52.Message, err error) error {
			if err != nil {
				fmt.Println(err)
			} else {
				fmt.Println("\n=====> Publish Complete")
				fmt.Println(ack)
			}
			return nil
		})
		require.NoError(t, err)
	}

	time.Sleep(100 * time.Millisecond)
	pubmsg := messagev52.NewPublishMessage()
	pubmsg.SetTopic([]byte("abc"))
	pubmsg.SetPayload([]byte("1234567890"))
	pubmsg.SetQoS(1)
	//pubmsg.SetTopicAlias(10)
	//pubmsg.SetNilTopicAndAlias(10)
	if pubmsg.QoS() > 0 {
		pubmsg.SetPacketId(uint16(atomic.AddUint32(&pkid, 1)))
	}
	fmt.Println("====== >>> Publish")
	err = c.Publish(pubmsg, func(msg, ack messagev52.Message, err error) error {
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println("\n=====> Publish Complete")
			fmt.Println(ack)
		}
		return nil
	})
	require.NoError(t, err)

	// Disconnects from the server
	time.Sleep(5 * time.Second)
	fmt.Println("\n====== >>> Disconnect")
	c.Disconnect()
	time.Sleep(100 * time.Millisecond)
}

func TestExampleClientSub(t *testing.T) {
	logger2.LogInit("debug") // 日志必须提前初始化

	// Instantiates a new Client
	c := &Client{}

	vp := authplus.NewDefaultAuth()
	c.AuthPlus = vp
	// Creates a new MQTT CONNECT messagev5 and sets the proper parameters
	msg := messagev52.NewConnectMessage()
	msg.SetWillQos(1)
	msg.SetVersion(5)
	msg.SetCleanSession(true)
	msg.SetClientId([]byte("surgemq2"))
	msg.SetKeepAlive(30)
	msg.SetWillTopic([]byte("will"))
	msg.SetWillMessage([]byte("send me home"))
	msg.SetUsername([]byte("surgemq"))
	msg.SetPassword([]byte("verysecret"))
	msg.SetAuthMethod([]byte("default"))
	msg.SetAuthData([]byte("aaa"))
	// Connects to the remote server at 127.0.0.1 port 1883
	err := c.Connect("tcp://127.0.0.1:1884", msg)
	require.NoError(t, err)
	time.Sleep(10 * time.Millisecond)
	// Creates a new SUBSCRIBE messagev5 to subscribe to topic "abc"
	submsg := messagev52.NewSubscribeMessage()
	submsg.AddTopic([]byte("abc"), 2)

	// Subscribes to the topic by sending the messagev5. The first nil in the function
	// call is a OnCompleteFunc that should handle the SUBACK messagev5 from the server.
	// Nil means we are ignoring the SUBACK messages. The second nil should be a
	// OnPublishFunc that handles any messages send to the client because of this
	// subscription. Nil means we are ignoring any PUBLISH messages for this topic.
	fmt.Println("====== >>> Subscribe")
	err = c.Subscribe(submsg, func(msg, ack messagev52.Message, err error) error {
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println("\n====== >>> Subscribe msg handle")
			fmt.Println(ack)
		}
		return nil
	}, func(msg *messagev52.PublishMessage, sub topics.Sub, sender string, shareMsg bool) error {
		fmt.Println("===<<<>>>", msg.String())
		if msg.SubscriptionIdentifier() > 0 {
			// ...
		}
		if len(msg.ResponseTopic()) > 0 {
			// ... 发送到响应主题去
		}
		return nil
	})
	require.NoError(t, err)
	time.Sleep(10 * time.Second)

	fmt.Println("====== >>> Unsubscribe")
	unsubmsg := messagev52.NewUnsubscribeMessage()
	unsubmsg.AddTopic([]byte("abc"))
	err = c.Unsubscribe(unsubmsg, func(msg, ack messagev52.Message, err error) error {
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println("\n====== >>> Unsubscribe handle")
			fmt.Println(ack)
		}
		return nil
	})
	require.NoError(t, err)
	// Disconnects from the server
	time.Sleep(4 * time.Second)
	fmt.Println("\n====== >>> Disconnect")
	c.Disconnect()
}