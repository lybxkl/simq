package impl

//import (
//	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
//	"testing"
//
//	"github.com/stretchr/testify/require"
//)
//
//func TestAckQueueOutOfOrder(t *testing.T) {
//	q := newAckqueue(5)
//	require.Equal(t, 8, q.cap())
//
//	for i := 0; i < 12; i++ {
//		msg := newPublishMessage(uint16(i), 1)
//		q.Wait(msg, nil)
//	}
//
//	require.Equal(t, 12, q.len())
//
//	ack1 := messagev5.NewPubackMessage()
//	ack1.SetPacketId(1)
//	q.Ack(ack1)
//
//	acked := q.Acked()
//
//	require.Equal(t, 0, len(acked))
//
//	ack0 := messagev5.NewPubackMessage()
//	ack0.SetPacketId(0)
//	q.Ack(ack0)
//
//	acked = q.Acked()
//
//	require.Equal(t, 2, len(acked))
//}
