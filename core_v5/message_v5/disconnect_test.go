package message

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func TestDisconnectMessageDecode(t *testing.T) {
	msgBytes := []byte{
		byte(DISCONNECT << 4),
		0,
	}

	msg := NewDisconnectMessage()
	n, err := msg.Decode(msgBytes)

	require.NoError(t, err, "Error decoding message.")
	require.Equal(t, len(msgBytes), n, "Error decoding message.")
	require.Equal(t, DISCONNECT, msg.Type(), "Error decoding message.")
}

func TestDisconnectMessageEncode(t *testing.T) {
	msgBytes := []byte{
		byte(DISCONNECT << 4),
		0,
	}

	msg := NewDisconnectMessage()

	dst := make([]byte, 10)
	n, err := msg.Encode(dst)

	require.NoError(t, err, "Error decoding message.")
	require.Equal(t, len(msgBytes), n, "Error decoding message.")
	require.Equal(t, msgBytes, dst[:n], "Error decoding message.")
}

// test to ensure encoding and decoding are the same
// decode, encode, and decode again
func TestDisconnectDecodeEncodeEquiv(t *testing.T) {
	msgBytes := []byte{
		byte(DISCONNECT << 4),
		0,
	}

	msg := NewDisconnectMessage()
	n, err := msg.Decode(msgBytes)

	require.NoError(t, err, "Error decoding message.")
	require.Equal(t, len(msgBytes), n, "Error decoding message.")

	dst := make([]byte, 100)
	n2, err := msg.Encode(dst)

	require.NoError(t, err, "Error decoding message.")
	require.Equal(t, len(msgBytes), n2, "Error decoding message.")
	require.Equal(t, msgBytes, dst[:n2], "Error decoding message.")

	n3, err := msg.Decode(dst)

	require.NoError(t, err, "Error decoding message.")
	require.Equal(t, len(msgBytes), n3, "Error decoding message.")
}
