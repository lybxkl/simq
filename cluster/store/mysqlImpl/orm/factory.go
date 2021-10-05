package orm

import (
	"context"
)

type Select map[string]interface{}

type SiOrm interface {
	AutoMigrate(dest interface{}) error
	// Save key 主键名称， value 主键值，msg可消息可session，key为空则为直接插入，不为空则为有则更新无则新增
	// 当msg中有主键则,key如果也是主键的话，就可以不用value了
	// 当单纯想新增一条数据，直接key=空
	Save(ctx context.Context, tab string, key string, value string, msg interface{}) error
	// SaveMany k 是不同订阅主题的条件，就是满足此条件，则更新，不满足则新增，msg 是对应的主题数据
	SaveMany(ctx context.Context, tab string, k []string, msg []interface{}) error
	// Get where 为空获取全部数据， decoder可为slice或者struct 的指针
	Get(ctx context.Context, tab string, where string, decoder interface{}) error
	// Delete where 删除条件，value只是填充使用，结构体指针
	Delete(ctx context.Context, tab string, where string, value interface{}) error
	// GetAndDelete decoder 为struct指针，value只是填充使用，结构体指针
	GetAndDelete(ctx context.Context, tab string, where string, decoder interface{}, value interface{}) error
}
