package sharetopic

// 集群共享主题数据
type ClusterShareTopicData interface {
	GetData() interface{}
	// 返回不同共享名称组应该发送给哪个节点的数据
	// 返回节点名称：节点需要发送的共享组名称
	SelectShare() map[string][]string
}

type ShareTopic interface {
	// 新增一个topic下某个shareName的订阅
	// A：$share/a_b/c
	// B：$share/a/b_c
	// 如果采用非/,+,#的拼接符
	// 会出现redis中冲突的情况，
	// 可以考虑换一个拼接符 '/'，因为$share/{shareName}/{filter} 中shareName中不能出现'/'的
	// 上述已修改为 '/'
	SubShare(topic, shareName, nodeName string) bool
	UnSubShare(topic, shareName, nodeName string) bool // 取消一个topic下某个shareName的订阅
	GetTopicShare(topic string) (ClusterShareTopicData, error)
	DelTopic(topic string) error // 删除主题
	// 删除节点相关订阅数据
	// 思路：获取当前节点的share manage，查询哪写需要删除，然后一次性删除
	DelNode(old map[string][]string, nodeName string) error
}
