package topics

import (
	"SI-MQTT/core/topics/share"
	"SI-MQTT/core/topics/sys"
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
