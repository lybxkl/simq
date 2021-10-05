package mongorepo

import (
	"context"
	"gitee.com/Ljolan/si-mqtt/cluster/common"
	store2 "gitee.com/Ljolan/si-mqtt/cluster/store"
	"gitee.com/Ljolan/si-mqtt/cluster/store/mongoImpl/orm"
	"gitee.com/Ljolan/si-mqtt/cluster/store/mongoImpl/orm/mongo"
	"gitee.com/Ljolan/si-mqtt/cluster/store/mongoImpl/po"
	"gitee.com/Ljolan/si-mqtt/config"
)

type eventStore struct {
	c orm.SiOrm
}

func NewEventStore() store2.EventStore {
	return &eventStore{}
}
func (e *eventStore) Start(ctx context.Context, config config.SIConfig) error {
	var err error
	storeCfg := config.Store.Mongo
	e.c, err = mongoorm.NewMongoOrm(storeCfg.Source, storeCfg.MinPoolSize, storeCfg.MaxPoolSize, storeCfg.MaxConnIdleTime)
	return err
}

func (e *eventStore) Stop(ctx context.Context) error {
	return nil
}

func (e *eventStore) PubEvent(ctx context.Context, event common.Event) error {
	return e.c.Save(ctx, "si_event", "", &event)
}

func (e *eventStore) GetEvent(ctx context.Context, maxEventTime int64) ([]common.Event, error) {
	data := make([]po.Event, 0)
	err := e.c.Get(ctx, "si_event", orm.Select{"time": orm.Select{"$gt": maxEventTime}}, &data)
	if err != nil || len(data) == 0 {
		return nil, err
	}
	ret := make([]common.Event, len(data))
	for i := 0; i < len(ret); i++ {
		ret[i] = common.Event(data[i])
	}
	return ret, nil
}

func (e *eventStore) GetEventMaxTime(ctx context.Context) (int64, error) {
	ret := &po.Event{}
	err := e.c.GetEnd(ctx, "si_event", orm.Select{}, "time", ret)
	if err != nil {
		return 0, err
	}
	return ret.Time, nil
}
