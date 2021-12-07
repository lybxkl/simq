// 共享订阅
package sys

import (
	messagev52 "gitee.com/Ljolan/si-mqtt/corev5/v2/message"
	topicsv52 "gitee.com/Ljolan/si-mqtt/corev5/v2/topics"
)

// TopicsProvider
type TopicsProvider interface {
	Subscribe(subs topicsv52.Sub, subscriber interface{}) (byte, error)
	Unsubscribe(topic []byte, subscriber interface{}) error
	Subscribers(topic []byte, qos byte, subs *[]interface{}, qoss *[]topicsv52.Sub) error
	Retain(msg *messagev52.PublishMessage) error
	Retained(topic []byte, msgs *[]*messagev52.PublishMessage) error
	Close() error
}
