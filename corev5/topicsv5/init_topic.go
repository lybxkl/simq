package topicsv5

import (
	"gitee.com/Ljolan/si-mqtt/corev5/topicsv5/share"
	"gitee.com/Ljolan/si-mqtt/corev5/topicsv5/sys"
)

func TopicInit(topicPro string) {
	switch topicPro {
	default:
		sys.SysTopicInit()
		share.ShareTopicInit()
		memTopicInit() // 这个顺序必须在前两个后面
	}
	//redisTopicInit()
}
