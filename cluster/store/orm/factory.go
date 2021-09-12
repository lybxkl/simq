package orm

import (
	"context"
)

type Select map[string]interface{}

type SiOrm interface {
	Save(ctx context.Context, tab string, key string, msg interface{}) error
	SaveMany(ctx context.Context, tab string, k []map[string]interface{}, msg []interface{}) error
	Get(ctx context.Context, tab string, sc Select, decoder interface{}) error
	GetEnd(ctx context.Context, tab string, sc Select, endKey string, decoder interface{}) error
	Delete(ctx context.Context, tab string, dc Select) error
	GetAndDelete(ctx context.Context, tab string, sc Select, decoder interface{}) error
	AddNum(ctx context.Context, tab string, sc Select, num Select) error
}
