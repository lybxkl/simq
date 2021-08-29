package colong

import (
	"github.com/apache/dubbo-getty"
	"sync"
)

const (
	CronPeriod      = 20e9
	WritePkgTimeout = 1e8
)

var log = getty.GetLogger()

var (
	pingresp []byte
	ack      []byte
	cs       sync.Map
	Cname    = "name"
	Caddr    = "addr"
)
