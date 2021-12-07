package sessions

//
//import (
//	"testing"
//
//	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
//	"github.com/stretchr/testify/require"
//)
//
//func TestSessionInit(t *testing.T) {
//	sess := &Session{}
//	cmsg := newConnectMessage()
//
//	err := sess.Init(cmsg)
//	require.NoError(t, err)
//	require.Equal(t, len(sess.cbuf), cmsg.Len())
//	require.Equal(t, cmsg.WillQos(), sess.Cmsg.WillQos())
//	require.Equal(t, cmsg.Version(), sess.Cmsg.Version())
//	require.Equal(t, cmsg.CleanSession(), sess.Cmsg.CleanSession())
//	require.Equal(t, cmsg.ClientId(), sess.Cmsg.ClientId())
//	require.Equal(t, cmsg.KeepAlive(), sess.Cmsg.KeepAlive())
//	require.Equal(t, cmsg.WillTopic(), sess.Cmsg.WillTopic())
//	require.Equal(t, cmsg.WillMessage(), sess.Cmsg.WillMessage())
//	require.Equal(t, cmsg.Username(), sess.Cmsg.Username())
//	require.Equal(t, cmsg.Password(), sess.Cmsg.Password())
//	require.Equal(t, []byte("will"), sess.Will.Topic())
//	require.Equal(t, cmsg.WillQos(), sess.Will.QoS())
//
//	sess.AddTopic("test", 1)
//	require.Equal(t, 1, len(sess.topics))
//
//	topics, qoss, err := sess.Topics()
//	require.NoError(t, err)
//	require.Equal(t, 1, len(topics))
//	require.Equal(t, 1, len(qoss))
//	require.Equal(t, "test", topics[0])
//	require.Equal(t, 1, int(qoss[0]))
//
//	sess.RemoveTopic("test")
//	require.Equal(t, 0, len(sess.topics))
//}
//
//func TestSessionPublishAckqueue(t *testing.T) {
//	sess := &Session{}
//	cmsg := newConnectMessage()
//	err := sess.Init(cmsg)
//	require.NoError(t, err)
//
//	for i := 0; i < 12; i++ {
//		msg := newPublishMessage(uint16(i), 1)
//		sess.Pub1ack.Wait(msg, nil)
//	}
//
//	require.Equal(t, 12, sess.Pub1ack.len())
//
//	ack1 := messagev5.NewPubackMessage()
//	ack1.SetPacketId(1)
//	sess.Pub1ack.Ack(ack1)
//
//	acked := sess.Pub1ack.Acked()
//	require.Equal(t, 0, len(acked))
//
//	ack0 := messagev5.NewPubackMessage()
//	ack0.SetPacketId(0)
//	sess.Pub1ack.Ack(ack0)
//
//	acked = sess.Pub1ack.Acked()
//	require.Equal(t, 2, len(acked))
//}
//
//func newConnectMessage() *messagev5.ConnectMessage {
//	msg := messagev5.NewConnectMessage()
//	msg.SetWillQos(1)
//	msg.SetVersion(4)
//	msg.SetCleanSession(true)
//	msg.SetClientId([]byte("surgemq"))
//	msg.SetKeepAlive(10)
//	msg.SetWillTopic([]byte("will"))
//	msg.SetWillMessage([]byte("send me home"))
//	msg.SetUsername([]byte("surgemq"))
//	msg.SetPassword([]byte("verysecret"))
//
//	return msg
//}
//
//func newPublishMessage(pktid uint16, qos byte) *messagev5.PublishMessage {
//	msg := messagev5.NewPublishMessage()
//	msg.SetPacketId(pktid)
//	msg.SetTopic([]byte("abc"))
//	msg.SetPayload([]byte("abc"))
//	msg.SetQoS(qos)
//
//	return msg
//}
