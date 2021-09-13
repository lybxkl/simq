package store

import (
	"context"
	"gitee.com/Ljolan/si-mqtt/cluster/common"
)

type EventStore interface {
	BaseStore
	PubEvent(ctx context.Context, event common.Event) error
	GetEvent(ctx context.Context, maxEventTime int64) ([]common.Event, error)
	GetEventMaxTime(ctx context.Context) (int64, error)
}
