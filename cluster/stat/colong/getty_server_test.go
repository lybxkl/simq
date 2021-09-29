package colong

import (
	"gitee.com/Ljolan/si-mqtt/cluster/stat/util"
	"testing"
)

func TestRunClusterServer(t *testing.T) {
	util.WaitCloseSignals(RunClusterGettyServer("node1", "127.0.0.1:9876",
		nil, nil, nil, nil))
}
