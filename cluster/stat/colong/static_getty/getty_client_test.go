package static_getty

import (
	"gitee.com/Ljolan/si-mqtt/cluster/stat/util"
	"testing"
)

func TestRunClient(t *testing.T) {
	util.WaitCloseSignals(runClient("node2", "node1", "127.0.0.1:9876", 1, true, 100))
}
