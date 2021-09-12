package store

import (
	"context"
	"gitee.com/Ljolan/si-mqtt/config"
)

type BaseStore interface {
	Start(ctx context.Context, config config.SIConfig) error
	Stop(ctx context.Context) error
}

type Store interface {
	SetStore(store SessionStore, messageStore MessageStore)
}
