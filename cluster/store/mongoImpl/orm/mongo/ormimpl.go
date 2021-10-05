package mongoorm

import (
	"context"
	"fmt"
	"gitee.com/Ljolan/si-mqtt/cluster/store/mongoImpl/orm"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo" //MongoDB的Go驱动包
	"go.mongodb.org/mongo-driver/mongo/options"
	"time"
)

type mongoOrm struct {
	db *mongo.Database
}

func NewMongoOrm(url string, minPool, maxPool, maxConnIdle uint64) (orm.SiOrm, error) {
	// 设置mongoDB客户端连接信息
	param := fmt.Sprintf(url)
	clientOptions := options.Client().
		ApplyURI(param).
		SetMinPoolSize(minPool).
		SetMaxPoolSize(maxPool).
		SetMaxConnIdleTime(time.Duration(maxConnIdle) * time.Second)

	// 建立客户端连接
	client, err := mongo.Connect(context.TODO(), clientOptions)
	if err != nil {
		return nil, err
	}

	// 检查连接情况
	err = client.Ping(context.TODO(), nil)
	if err != nil {
		return nil, err
	}
	fmt.Println("Connected to MongoDB!")

	cli := client.Database("simq")

	return &mongoOrm{db: cli}, nil
}
func (m *mongoOrm) Save(ctx context.Context, tab string, key string, msg interface{}) error {
	if key == "" {
		_, err := m.db.Collection(tab).InsertOne(ctx, msg)
		if err != nil {
			return err
		}
		return nil
	}
	update := bson.M{"$set": msg}
	updateOpts := options.Update().SetUpsert(true)
	filter := bson.M{"_id": key}
	_, err := m.db.Collection(tab).UpdateOne(ctx, filter, update, updateOpts)
	if err != nil {
		return err
	}
	return nil
}

// TODO 事务
func (m *mongoOrm) SaveMany(ctx context.Context, tab string, k []map[string]interface{}, msg []interface{}) error {
	if len(k) == 0 {
		_, err := m.db.Collection(tab).InsertMany(ctx, msg)
		if err != nil {
			return err
		}
		return nil
	}
	for i := 0; i < len(msg); i++ {
		update := bson.M{"$set": msg[i]}
		updateOpts := options.Update().SetUpsert(true)
		filter := bson.M(k[i])
		_, err := m.db.Collection(tab).UpdateOne(ctx, filter, update, updateOpts)
		if err != nil {
			return err
		}
	}
	return nil
}
func (m *mongoOrm) Get(ctx context.Context, tab string, sc orm.Select, decoder interface{}) error {
	c, err := m.db.Collection(tab).Find(ctx, bson.M(sc))
	if err != nil {
		return err
	}
	return c.All(ctx, decoder)
}

// GetEnd 获取最后一条，根据
func (m *mongoOrm) GetEnd(ctx context.Context, tab string, sc orm.Select, endKey string, decoder interface{}) error {
	op := &options.FindOneOptions{}
	op.SetSort(bson.M{endKey: -1})
	return m.db.Collection(tab).FindOne(ctx, bson.M(sc), op).Decode(decoder)
}
func (m *mongoOrm) GetAndDelete(ctx context.Context, tab string, sc orm.Select, decoder interface{}) error {
	c := m.db.Collection(tab).FindOneAndDelete(ctx, bson.M(sc))
	return c.Decode(decoder)
}
func (m *mongoOrm) Delete(ctx context.Context, tab string, dc orm.Select) error {
	_, err := m.db.Collection(tab).DeleteMany(ctx, dc)
	if err != nil {
		return err
	}
	return nil
}

func (m *mongoOrm) AddNum(ctx context.Context, tab string, sc orm.Select, num orm.Select) error {
	_, err := m.db.Collection(tab).UpdateOne(ctx, sc, num)
	if err != nil {
		return err
	}
	return nil
}
