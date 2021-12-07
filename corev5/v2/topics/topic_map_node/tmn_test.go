package topicmapnode

import (
	"fmt"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestSub(t *testing.T) {
	tmnp := NewTopicMapNodeProvider()
	tmnp.Subscribe([]byte("/a/s/v"), "as1", "node1", 1)
	tmnp.Subscribe([]byte("/a/s/v"), "as1", "node1", 100)
	tmnp.Subscribe([]byte("/a/s/v"), "as2", "node1", 1)
	tmnp.Subscribe([]byte("/a/s/v"), "as1", "node2", 1)
	tmnp.Subscribe([]byte("/a/s/v"), "as2", "node2", 1)
	tmnp.Subscribe([]byte("/a/s/v"), "as1", "node2", 1)
	tmnp.Subscribe([]byte("/a/s/v2"), "as1", "node1", 1)
	tmnp.Subscribe([]byte("/a/s/v2"), "as1", "node2", 1)
	s, n := make([]string, 0), make([]string, 0)
	tag := make(map[string]map[string]int)
	//tmnp.Unsubscribe([]byte("/a/s/v"), "as2", "node1")
	for i := 0; i < 10000; i++ {
		err := tmnp.Subscribers([]byte("/a/s/v"), &s, &n)
		require.NoError(t, err)
		for j := 0; j < len(s); j++ {
			var ta map[string]int
			var ok bool
			if ta, ok = tag[s[j]]; !ok {
				ta = make(map[string]int)
			}
			ta[n[j]]++
			tag[s[j]] = ta
		}
		s = s[0:0]
		n = n[0:0]
	}
	fmt.Println(tag)
}
