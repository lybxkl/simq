package mongorepo

import (
	"context"
	"fmt"
	"gitee.com/Ljolan/si-mqtt/config"
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
	"github.com/stretchr/testify/require"
	"testing"
)

// SI_CFG_PATH=F:/Go_pro/src/si-mqtt/config
func TestMessgae(t *testing.T) {
	ms := NewMessageStore()
	ctx := context.Background()
	ms.Start(ctx, config.SIConfig{})

	pub := NewPub("/a/b/c")
	pub2 := NewPub("/a/b/c2")
	require.NoError(t, ms.StoreRetainMessage(ctx, "/a/b/c", pub))
	require.NoError(t, ms.StoreRetainMessage(ctx, "/a/b/c", pub2))
	require.NoError(t, ms.StoreRetainMessage(ctx, "/a/b/c2", pub2))
	fmt.Println(ms.GetRetainMessage(ctx, "/a/b/c"))
	fmt.Println(ms.GetAllRetainMsg(ctx))
	require.NoError(t, ms.ClearRetainMessage(ctx, "/a/b/c"))

	require.NoError(t, ms.StoreWillMessage(ctx, "/a/b/c", pub))
	require.NoError(t, ms.StoreWillMessage(ctx, "/a/b/c", pub2))
	require.NoError(t, ms.StoreWillMessage(ctx, "/a/b/c2", pub2))
	fmt.Println(ms.GetWillMessage(ctx, "/a/b/c"))
	require.NoError(t, ms.ClearWillMessage(ctx, "/a/b/c"))
}

func NewPub(topic string) *messagev5.PublishMessage {
	pub := messagev5.NewPublishMessage()
	pub.SetQoS(0x00)
	pub.SetPacketId(123)
	pub.SetContentType([]byte("type"))
	pub.SetCorrelationData([]byte("pk"))
	pub.SetResponseTopic([]byte(topic))
	pub.SetPayloadFormatIndicator(0x01)
	pub.SetPayload([]byte("11"))
	pub.AddUserPropertys([][]byte{[]byte("aaa:bb"), []byte("cc:ssd")})
	pub.SetMessageExpiryInterval(30)
	pub.SetTopic([]byte("11"))
	pub.SetTopicAlias(100)
	return pub
}
