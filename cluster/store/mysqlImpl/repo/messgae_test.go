package mysqlrepo

import (
	"context"
	"fmt"
	"gitee.com/Ljolan/si-mqtt/config"
	messagev2 "gitee.com/Ljolan/si-mqtt/corev5/v2/message"
	"github.com/stretchr/testify/require"
	"testing"
)

// SI_CFG_PATH=F:/Go_pro/src/si-mqtt/config
func TestMessgae(t *testing.T) {
	ms := NewMessageStore()
	ctx := context.Background()
	ms.Start(ctx, config.GetConfig())

	pub := NewPub("/a/b/c")
	pub2 := NewPub("/a/b/c2")
	pub3 := NewPub("/a/b/c")
	pub3.SetPayload([]byte("asdasda"))
	require.NoError(t, ms.StoreRetainMessage(ctx, "/a/b/c", pub))
	require.NoError(t, ms.StoreRetainMessage(ctx, "/a/b/c", pub3))
	require.NoError(t, ms.StoreRetainMessage(ctx, "/a/b/c2", pub2))
	fmt.Println(ms.GetRetainMessage(ctx, "/a/b/c"))
	fmt.Println(ms.GetAllRetainMsg(ctx))
	require.NoError(t, ms.ClearRetainMessage(ctx, "/a/b/c"))

	require.NoError(t, ms.StoreWillMessage(ctx, "c1", pub))
	require.NoError(t, ms.StoreWillMessage(ctx, "c1", pub2))
	require.NoError(t, ms.StoreWillMessage(ctx, "c2", pub2))
	fmt.Println(ms.GetWillMessage(ctx, "c1"))
	require.NoError(t, ms.ClearWillMessage(ctx, "c1"))
}

func NewPub(topic string) *messagev2.PublishMessage {
	pub := messagev2.NewPublishMessage()
	pub.SetQoS(0x00)
	pub.SetPacketId(123)
	pub.SetContentType([]byte("type"))
	pub.SetCorrelationData([]byte("pk"))
	pub.SetResponseTopic([]byte(topic))
	pub.SetPayloadFormatIndicator(0x01)
	pub.SetPayload([]byte("11"))
	pub.AddUserPropertys([][]byte{[]byte("aaa:bb"), []byte("cc:ssd")})
	pub.SetMessageExpiryInterval(30)
	pub.SetTopic([]byte(topic))
	pub.SetTopicAlias(100)
	return pub
}
