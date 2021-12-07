package topicmapnode

// TopicsMapNodeProvider 主题映射节点名称
type TopicsMapNodeProvider interface {
	Subscribe(topic []byte, shareName, node string, num uint32) error
	Unsubscribe(topic []byte, shareName, node string) error
	Subscribers(topic []byte, shareNames, nodes *[]string) error // 获取此主题下不同共享组中选中的节点
	Close() error
}
