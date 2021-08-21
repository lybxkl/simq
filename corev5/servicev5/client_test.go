package servicev5

import (
	"fmt"
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"gitee.com/Ljolan/si-mqtt/corev5/topicsv5"
	logger2 "gitee.com/Ljolan/si-mqtt/logger"
	"github.com/stretchr/testify/require"
	"testing"
	"time"
)

func TestExampleClient(t *testing.T) {
	logger2.LogInit("debug") // 日志必须提前初始化
	topicsv5.TopicInit("")
	// Instantiates a new Client
	c := &Client{}

	// Creates a new MQTT CONNECT messagev5 and sets the proper parameters
	msg := messagev5.NewConnectMessage()
	msg.SetWillQos(1)
	msg.SetVersion(5)
	msg.SetCleanSession(true)
	msg.SetClientId([]byte("surgemq"))
	msg.SetKeepAlive(1000)
	msg.SetWillTopic([]byte("will"))
	msg.SetWillMessage([]byte("send me home"))
	msg.SetUsername([]byte("surgemq"))
	msg.SetPassword([]byte("verysecret"))

	// Connects to the remote server at 127.0.0.1 port 1883
	err := c.Connect("tcp://127.0.0.1:1883", msg)
	require.NoError(t, err)

	// Creates a new SUBSCRIBE messagev5 to subscribe to topic "abc"
	submsg := messagev5.NewSubscribeMessage()
	submsg.AddTopic([]byte("abc"), 0)

	// Subscribes to the topic by sending the messagev5. The first nil in the function
	// call is a OnCompleteFunc that should handle the SUBACK messagev5 from the server.
	// Nil means we are ignoring the SUBACK messages. The second nil should be a
	// OnPublishFunc that handles any messages send to the client because of this
	// subscription. Nil means we are ignoring any PUBLISH messages for this topic.
	fmt.Println("====== >>> Subscribe")
	err = c.Subscribe(submsg, func(msg, ack messagev5.Message, err error) error {
		fmt.Println("====== >>> Subscribe msg handle")
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(msg)
			fmt.Println("=====>")
			fmt.Println(ack)
		}
		return nil
	}, func(msg *messagev5.PublishMessage) error {
		fmt.Println(msg.String())
		return nil
	})
	require.NoError(t, err)
	// Creates a new PUBLISH messagev5 with the appropriate contents for publishing
	pubmsg := messagev5.NewPublishMessage()
	pubmsg.SetTopic([]byte("abc"))
	pubmsg.SetPayload(make([]byte, 10))
	pubmsg.SetQoS(0)

	// Publishes to the server by sending the messagev5
	fmt.Println("====== >>> Publish")
	err = c.Publish(pubmsg, func(msg, ack messagev5.Message, err error) error {
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(msg)
			fmt.Println("=====>")
			fmt.Println(ack)
		}
		return nil
	})
	require.NoError(t, err)

	fmt.Println("====== >>> Unsubscribe")
	unsubmsg := messagev5.NewUnsubscribeMessage()
	unsubmsg.AddTopic([]byte("abc"))
	err = c.Unsubscribe(unsubmsg, func(msg, ack messagev5.Message, err error) error {
		fmt.Println("====== >>> Unsubscribe handle")
		if err != nil {
			fmt.Println(err)
		} else {
			fmt.Println(msg)
			fmt.Println("=====>")
			fmt.Println(ack)
		}
		return nil
	})
	require.NoError(t, err)
	// Disconnects from the server
	time.Sleep(100 * time.Millisecond)
	fmt.Println("====== >>> Disconnect")
	c.Disconnect()
}
