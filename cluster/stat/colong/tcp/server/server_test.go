package server

import (
	"gitee.com/Ljolan/si-mqtt/cluster/stat/util"
	"testing"
)

func TestRunClusterServer(t *testing.T) {
	util.WaitCloseSignals(RunClusterServer("node1", "127.0.0.1:9876",
		nil, nil, nil, nil))
}
