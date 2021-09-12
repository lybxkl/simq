package repo

import (
	"context"
	"fmt"
	"gitee.com/Ljolan/si-mqtt/config"
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"gitee.com/Ljolan/si-mqtt/corev5/sessionsv5/impl"
	"github.com/stretchr/testify/require"
	"testing"
)

// SI_CFG_PATH=F:/Go_pro/src/si-mqtt/config
func TestSessionStore(t *testing.T) {
	ss := NewSessionStore()
	ctx := context.Background()
	ss.Start(ctx, config.SIConfig{})
	clientId := "aa"

	msg := messagev5.NewConnectMessage()
	msg.SetClientId([]byte(clientId))
	msg.SetCleanSession(true)
	msg.SetWillFlag(true)
	msg.SetWillQos(0x00)
	msg.SetWillRetain(false)
	msg.SetUsernameFlag(true)
	msg.SetPasswordFlag(true)
	msg.SetVersion(0x05)
	msg.SetKeepAlive(100)
	msg.SetClientId([]byte("aaaaaasssss"))
	msg.SetWillTopic([]byte("willtp1"))
	msg.SetWillMessage([]byte("will ploay"))
	msg.SetUsername([]byte("name"))
	msg.SetPassword([]byte("pwd"))
	msg.SetSessionExpiryInterval(132)
	msg.SetReceiveMaximum(900)
	msg.SetMaxPacketSize(100)
	msg.SetTopicAliasMax(20)
	msg.SetRequestRespInfo(0)
	msg.SetRequestProblemInfo(1)
	msg.AddUserPropertys([][]byte{[]byte("a4221d"), []byte("ssss")})
	msg.AddWillUserPropertys([][]byte{[]byte("cccc"), []byte("saaa")})
	msg.SetAuthMethod([]byte("abcd"))
	msg.SetAuthData([]byte("abcd"))
	msg.SetWillDelayInterval(30)
	msg.SetPayloadFormatIndicator(1)
	msg.SetWillMsgExpiryInterval(20)
	msg.SetContentType([]byte("type"))
	msg.SetResponseTopic([]byte("/a/p/9"))
	msg.SetCorrelationData([]byte("db"))

	require.NoError(t, ss.StoreSession(ctx, clientId, impl.NewMemSessionByCon(msg)))
	s, err := ss.GetSession(ctx, clientId)
	require.NoError(t, err)
	fmt.Println(s.Cmsg())

	pub := messagev5.NewPublishMessage()
	pub.SetQoS(0x00)
	pub.SetPacketId(123)
	pub.SetContentType([]byte("type"))
	pub.SetCorrelationData([]byte("pk"))
	pub.SetResponseTopic([]byte("/a/b/c"))
	pub.SetPayloadFormatIndicator(0x01)
	pub.SetPayload([]byte("11"))
	pub.AddUserPropertys([][]byte{[]byte("aaa:bb"), []byte("cc:ssd")})
	pub.SetMessageExpiryInterval(30)
	pub.SetTopic([]byte("11"))
	pub.SetTopicAlias(100)

	require.NoError(t, ss.CacheInflowMsg(ctx, clientId, pub))
	pub.SetPacketId(124)
	require.NoError(t, ss.CacheInflowMsg(ctx, clientId, pub))
	pub.SetPacketId(123)
	fmt.Println(ss.GetAllInflowMsg(ctx, clientId))
	fmt.Println(ss.ReleaseInflowMsg(ctx, clientId, 123))

	require.NoError(t, ss.CacheOutflowMsg(ctx, clientId, pub))
	pub.SetPacketId(124)
	require.NoError(t, ss.CacheOutflowMsg(ctx, clientId, pub))
	pub.SetPacketId(123)
	fmt.Println(ss.GetAllOutflowMsg(ctx, clientId))
	fmt.Println(ss.ReleaseOutflowMsg(ctx, clientId, 123))

	require.NoError(t, ss.CacheOutflowSecMsgId(ctx, clientId, 123))
	require.NoError(t, ss.CacheOutflowSecMsgId(ctx, clientId, 124))
	fmt.Println(ss.GetAllOutflowSecMsg(ctx, clientId))
	require.NoError(t, ss.ReleaseOutflowSecMsgId(ctx, clientId, 123))
	//ss.ReleaseOutflowSecMsgId(ctx, clientId, 124)

	require.NoError(t, ss.StoreOfflineMsg(ctx, clientId, pub))
	require.NoError(t, ss.StoreOfflineMsg(ctx, clientId, pub))
	require.NoError(t, ss.StoreOfflineMsg(ctx, clientId, pub))
	sof, ps, _ := ss.GetAllOfflineMsg(ctx, clientId)
	fmt.Println(sof)
	require.NoError(t, ss.ClearOfflineMsgById(ctx, clientId, ps[:1]))
	sof, _, _ = ss.GetAllOfflineMsg(ctx, clientId)
	fmt.Println(sof)
	require.NoError(t, ss.ClearOfflineMsgs(ctx, clientId))
	sof, _, _ = ss.GetAllOfflineMsg(ctx, clientId)
	fmt.Println(sof)

	sub := messagev5.NewSubscribeMessage()
	sub.SetPacketId(100)
	sub.AddTopic([]byte("/a/b/#/c"), 1)
	sub.SetTopicLocal([]byte("/a/b/#/c"), true)
	//sub.RemoveTopic([]byte("/a/b/#/c"))
	sub.AddTopic([]byte("/a/b/#/c2"), 1)
	sub.AddTopic([]byte("/a/b/#/c3"), 1)
	sub.SetTopicLocal([]byte("/a/b/#/c2"), false)
	sub.SetTopicRetainAsPublished([]byte("/a/b/#/c"), false)
	sub.SetTopicRetainHandling([]byte("/a/b/#/c"), 0)
	sub.AddUserPropertys([][]byte{[]byte("asd"), []byte("ccc:sa")})
	sub.SetSubscriptionIdentifier(123)

	require.NoError(t, ss.StoreSubscription(ctx, clientId, sub))
	fmt.Println(ss.GetSubscriptions(ctx, clientId))
	require.NoError(t, ss.DelSubscription(ctx, clientId, "/a/b/#/c"))
	require.NoError(t, ss.ClearSubscriptions(ctx, clientId))
	//ss.ClearSession(ctx, clientId, false)
	//s, err = ss.GetSession(ctx, clientId)
	//require.NoError(t, err)
	//fmt.Println(s)
}
