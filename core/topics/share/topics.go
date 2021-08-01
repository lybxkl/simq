// 共享订阅
package share

import (
	"SI-MQTT/core/logger"
	"SI-MQTT/core/message"
	"errors"
	"fmt"
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

	providers = make(map[string]ShareTopicsProvider)
)

// TopicsProvider
type ShareTopicsProvider interface {
	Subscribe(topic, shareName []byte, qos byte, subscriber interface{}) (byte, error)
	Unsubscribe(topic, shareName []byte, subscriber interface{}) error
	Subscribers(topic, shareName []byte, qos byte, subs *[]interface{}, qoss *[]byte) error
	AllSubInfo() (map[string][]string, error) // 获取所有的共享订阅，k: 主题，v: 该主题的所有共享组
	Retain(msg *message.PublishMessage, shareName []byte) error
	Retained(topic, shareName []byte, msgs *[]*message.PublishMessage) error
	Close() error
}

func Register(name string, provider ShareTopicsProvider) {
	if provider == nil {
		panic("share topics: Register provide is nil")
	}

	if _, dup := providers[name]; dup {
		panic("share topics: Register called twice for provider " + name)
	}

	providers[name] = provider
	logger.Logger.Info(fmt.Sprintf("Register Share TopicsProvider：'%s' success，%T", name, provider))
}

func Unregister(name string) {
	delete(providers, name)
}

type Manager struct {
	p ShareTopicsProvider
}

func NewManager(providerName string) (*Manager, error) {
	p, ok := providers[providerName]
	if !ok {
		return nil, fmt.Errorf("session: unknown provider %q", providerName)
	}

	return &Manager{p: p}, nil
}

func (this *Manager) Subscribe(topic, shareName []byte, qos byte, subscriber interface{}) (byte, error) {
	return this.p.Subscribe(topic, shareName, qos, subscriber)
}

func (this *Manager) AllSubInfo() (map[string][]string, error) {
	return this.p.AllSubInfo()
}

func (this *Manager) Unsubscribe(topic, shareName []byte, subscriber interface{}) error {
	return this.p.Unsubscribe(topic, shareName, subscriber)
}

func (this *Manager) Subscribers(topic, shareName []byte, qos byte, subs *[]interface{}, qoss *[]byte) error {
	return this.p.Subscribers(topic, shareName, qos, subs, qoss)
}

func (this *Manager) Retain(msg *message.PublishMessage, shareName []byte) error {
	return this.p.Retain(msg, shareName)
}

func (this *Manager) Retained(topic, shareName []byte, msgs *[]*message.PublishMessage) error {
	return this.p.Retained(topic, shareName, msgs)
}

func (this *Manager) Close() error {
	return this.p.Close()
}
