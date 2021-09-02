package client

import (
	"gitee.com/Ljolan/si-mqtt/cluster/stat/util"
	"testing"
)

func TestRunClient(t *testing.T) {
	util.WaitCloseSignals(RunClient("node2", "node1", "127.0.0.1:9876", 1, true, 100))
}
