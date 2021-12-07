package po

import (
	"gitee.com/Ljolan/si-mqtt/corev5/v2/sessions"
)

// Session 会话
type Session struct {
	ClientId string          `gorm:"column:client_id;primary_key"`
	Status   sessions.Status `gorm:"column:status"`

	ExpiryInterval     uint32 `gorm:"column:expiry_interval"`      // session 过期时间
	ReceiveMaximum     uint16 `gorm:"column:receive_maximum"`      // 接收最大值
	MaxPacketSize      uint32 `gorm:"column:max_packet_size"`      // 最大报文长度是 MQTT 控制报文的总长度
	TopicAliasMax      uint16 `gorm:"column:topic_alias_max"`      // 主题别名最大值（Topic Alias Maximum）
	RequestRespInfo    byte   `gorm:"column:request_resp_info"`    // 一个字节表示的 0 或 1, 请求响应信息
	RequestProblemInfo byte   `gorm:"column:request_problem_info"` // 一个字节表示的 0 或 1 , 请求问题信息
	UserProperty       string `gorm:"column:user_property"`        // 用户属性，可变报头的 []string--> hex string join ,

	//responseTopic          []byte   // 响应主题
	//correlationData        []byte   // 对比数据
	OfflineTime int64 `gorm:"column:offline_time"`
}

func (w Session) TableName() string {
	return "si_session"
}
