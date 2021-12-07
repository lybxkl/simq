package po

import (
	"gitee.com/Ljolan/si-mqtt/corev5/v2/sessions"
)

// Session 会话
type Session struct {
	ClientId string          `bson:"_id"`
	Status   sessions.Status `bson:"status"`

	ExpiryInterval     uint32   `bson:"expiry_interval"`      // session 过期时间
	ReceiveMaximum     uint16   `bson:"receive_maximum"`      // 接收最大值
	MaxPacketSize      uint32   `bson:"max_packet_size"`      // 最大报文长度是 MQTT 控制报文的总长度
	TopicAliasMax      uint16   `bson:"topic_alias_max"`      // 主题别名最大值（Topic Alias Maximum）
	RequestRespInfo    byte     `bson:"request_resp_info"`    // 一个字节表示的 0 或 1, 请求响应信息
	RequestProblemInfo byte     `bson:"request_problem_info"` // 一个字节表示的 0 或 1 , 请求问题信息
	UserProperty       []string `bson:"user_property"`        // 用户属性，可变报头的

	//responseTopic          []byte   // 响应主题
	//correlationData        []byte   // 对比数据
	OfflineTime int64 `bson:"offline_time"`
}
