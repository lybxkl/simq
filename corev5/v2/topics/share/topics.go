// 共享订阅
package share

import (
	messagev2 "gitee.com/Ljolan/si-mqtt/corev5/v2/message"
	"gitee.com/Ljolan/si-mqtt/corev5/v2/topics"
)

type TopicsProvider interface {
	Subscribe(shareName []byte, sub topics.Sub, subscriber interface{}) (byte, error)
	Unsubscribe(topic, shareName []byte, subscriber interface{}) error
	Subscribers(topic, shareName []byte, qos byte, subs *[]interface{}, qoss *[]topics.Sub) error
	AllSubInfo() (map[string][]string, error) // 获取所有的共享订阅，k: 主题，v: 该主题的所有共享组
	Retain(msg *messagev2.PublishMessage, shareName []byte) error
	Retained(topic, shareName []byte, msgs *[]*messagev2.PublishMessage) error
	Close() error
}
