package topicmapnode

const (
	// MWC is the multi-level wildcard
	MWC = "#"

	// SWC is the single level wildcard
	SWC = "+"

	// SEP is the topic level separator
	SEP = "/"

	// SYS is the starting character of the system level topics
	//SYS是系统级主题的起始字符
	SYS = "$"

	// Both wildcards
	_WC = "#+"
)

// TopicsMapNodeProvider 主题映射节点名称
type TopicsMapNodeProvider interface {
	Subscribe(topic []byte, shareName, node string, num uint32) error
	Unsubscribe(topic []byte, shareName, node string) error
	Subscribers(topic []byte, shareNames, nodes *[]string) error // 获取此主题下不同共享组中选中的节点
	Close() error
}
