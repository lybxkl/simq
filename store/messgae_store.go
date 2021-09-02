package store

import (
	"gitee.com/Ljolan/si-mqtt/config"
	"gitee.com/Ljolan/si-mqtt/corev5/messagev5"
)

type MessageStore interface {
	Start(config config.SIConfig) error
	Stop() error
	
	StoreWillMessage(clientId string, message messagev5.Message) error
	ClearWillMessage(clientId string) error
	GetWillMessage(clientId string) (messagev5.Message,error)

	StoreRetainMessage(topic string, message messagev5.Message) error
	ClearRetainMessage(topic string) error
	GetRetainMessage(topic string) (messagev5.Message,error)

	GetAllRetainMsg() ([]messagev5.Message,error)
}
