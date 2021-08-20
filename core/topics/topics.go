// Package topics deals with MQTT topic names, topic filters and subscriptions.
// - "Topic name" is a / separated string that could contain #, * and $
// - / in topic name separates the string into "topic levels"
// - # is a multi-level wildcard, and it must be the last character in the
//   topic name. It represents the parent and all children levels.
// - + is a single level wildwcard. It must be the only character in the
//   topic level. It represents all names in the current level.
// - $ is a special character that says the topic is a system level topic
package topics

import (
	"errors"
	"fmt"
	"gitee.com/Ljolan/si-mqtt/core/message"
	logger2 "gitee.com/Ljolan/si-mqtt/logger"
)

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

var (
	// ErrAuthFailure is returned when the user/pass supplied are invalid
	ErrAuthFailure = errors.New("auth: Authentication failure")

	// ErrAuthProviderNotFound is returned when the requested provider does not exist.
	// It probably hasn't been registered yet.
	ErrAuthProviderNotFound = errors.New("auth: Authentication provider not found")

	providers = make(map[string]TopicsProvider)
)

// TopicsProvider
type TopicsProvider interface {
	Subscribe(topic []byte, qos byte, subscriber interface{}) (byte, error)
	Unsubscribe(topic []byte, subscriber interface{}) error
	// svc 表示是服务端下发的数据，系统主题消息
	// shareName 为空表示不需要发送任何共享消息，不为空表示只需要发送当前shareName下的订阅者
	// 系统主题消息和共享主题消息，不能同时获取，系统主题优先于共享主题

	// if shareName == "" && onlyShare == false ===>> 表示不需要获取任何共享主题订阅者，只需要所有非共享组的订阅者们
	// if shareName == "" && onlyShare == true  ===>> 表示获取当前主题shareName的所有共享组每个的组的一个订阅者，不需要所有非共享组的订阅者们
	// if onlyShare == false && shareName != "" ===>> 获取当前主题的共享组名为shareName的订阅者一个与所有非共享组订阅者们
	// if onlyShare == true && shareName != ""  ===>> 仅仅获取主题的共享组名为shareName的订阅者一个
	Subscribers(topic []byte, qos byte, subs *[]interface{}, qoss *[]byte, svc bool, shareName string, onlyShare bool) error
	AllSubInfo() (map[string][]string, error) // 获取所有的共享订阅，k: 主题，v: 该主题的所有共享组
	Retain(msg *message.PublishMessage) error
	Retained(topic []byte, msgs *[]*message.PublishMessage) error
	Close() error
}

var Default = "default"

func Register(name string, provider TopicsProvider) {
	if provider == nil {
		panic("topics: Register provide is nil")
	}
	if name == "" {
		name = Default
	}
	if _, dup := providers[name]; dup {
		panic("topics: Register called twice for provider " + name)
	}

	providers[name] = provider
	logger2.Logger.Infof("Register TopicsProvider：'%s' success，%T", name, provider)
}

func Unregister(name string) {
	if name == "" {
		name = Default
	}
	delete(providers, name)
}

type Manager struct {
	p TopicsProvider
}

func NewManager(providerName string) (*Manager, error) {
	if providerName == "" {
		providerName = Default
	}
	p, ok := providers[providerName]
	if !ok {
		return nil, fmt.Errorf("session: unknown provider %q", providerName)
	}

	return &Manager{p: p}, nil
}

func (this *Manager) Subscribe(topic []byte, qos byte, subscriber interface{}) (byte, error) {
	return this.p.Subscribe(topic, qos, subscriber)
}

func (this *Manager) Unsubscribe(topic []byte, subscriber interface{}) error {
	return this.p.Unsubscribe(topic, subscriber)
}

// if shareName == "" && onlyShare == false ===>> 表示不需要获取任何共享主题订阅者，只需要所有非共享组的订阅者们
// if shareName == "" && onlyShare == true  ===>> 表示获取当前主题shareName的所有共享组每个的组的一个订阅者，不需要所有非共享组的订阅者们
// if onlyShare == false && shareName != "" ===>> 获取当前主题的共享组名为shareName的订阅者一个与所有非共享组订阅者们
// if onlyShare == true && shareName != ""  ===>> 仅仅获取主题的共享组名为shareName的订阅者一个
func (this *Manager) Subscribers(topic []byte, qos byte, subs *[]interface{}, qoss *[]byte, svc bool, shareName string, onlyShare bool) error {
	return this.p.Subscribers(topic, qos, subs, qoss, svc, shareName, onlyShare)
}

func (this *Manager) AllSubInfo() (map[string][]string, error) {
	return this.p.AllSubInfo()
}

func (this *Manager) Retain(msg *message.PublishMessage) error {
	return this.p.Retain(msg)
}

func (this *Manager) Retained(topic []byte, msgs *[]*message.PublishMessage) error {
	return this.p.Retained(topic, msgs)
}

func (this *Manager) Close() error {
	return this.p.Close()
}
