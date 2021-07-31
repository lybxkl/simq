package redis

import (
	"container/list"
	"encoding/json"
	"fmt"
	"github.com/garyburd/redigo/redis"
	"strconv"
	"time"
)

/**
* 自己想要其它的方法  ， 可以通过 redis.Do( (1), ...)
*  (1): redis的执行命令第一个参数如 ： set , lrang , delete
*  后面的... 是接在后面的参数，比如set的key , value  ：set ,"key" , "value"
* 参照 https://blog.csdn.net/wangshubo1989/article/details/75050024
**/
type Redis struct {
	rc redis.Conn
}

//创建连接
func (r *Redis) CreatCon(network, add, password string, db int) error {
	c, err := redis.Dial(network, add, redis.DialDatabase(db), redis.DialPassword(password))
	if err != nil {
		return err
	}
	r.rc = c
	return nil
}

//设置key value
func (r *Redis) SetV(key string, value interface{}) error {
	_, err := r.rc.Do("SET", key, value)
	if err != nil {
		return err
	}
	return nil
}

//可以充当分布式锁，只有不存在才会执行成功
func (r *Redis) SetNX(key string, value interface{}) bool {
	n, err := r.rc.Do("SETNX", key, value)
	if err != nil {
		fmt.Println(err)
		return false
	}
	if n == int64(1) {
		return true
	}
	fmt.Println(key, "has exited in redis,you did`t to set the key")
	return false
}

//设置key value 过期时间
func (r *Redis) SetVHasTime(key string, value interface{}, time int64) error {
	time = checkTime(time)
	_, err := r.rc.Do("SET", key, value, "EX", strconv.Itoa(int(time)))
	if err != nil {
		return err
	}
	return nil
}

//可以充当分布式锁，但是不会是死锁，到时间自动删除，只有不存在才会执行成功
func (r *Redis) SetNXHasTime(key string, value interface{}, time int64) bool {
	time = checkTime(time)
	n, err := r.rc.Do("SETNX", key, value, "EX", strconv.Itoa(int(time)))
	if err != nil {
		fmt.Println(err)
		return false
	}
	if n == int64(1) {
		return true
	}
	fmt.Println(key, "has exited in redis,you did`t to set the key")
	return false
}

// 获取对应key 的 value ，返回的是[]bytes ,还有其它返回类型，自动修改
func (r *Redis) GetV(key string) ([]byte, error) {
	n, err := redis.Bytes(r.rc.Do("GET", key))
	if err != nil {
		return nil, err
	}
	return n, nil
}

// 判断是否存在当前key
func (r *Redis) ExitsV(key string) bool {
	exit, err := redis.Bool(r.rc.Do("EXISTS", key))
	if err != nil {
		fmt.Println("exit method --> ", err)
	}
	return exit
}

// 删除key
func (r *Redis) DeleteV(key string) bool {
	n, err := r.rc.Do("DEL", key)
	if err != nil {
		fmt.Println("redis delelte failed:", err)
		return false
	}
	if n.(int64) <= 0 {
		fmt.Println("redis delelte failed，no this key:", n)
		return false
	}
	fmt.Println(n)
	return true
}

// 为某个key设置过期时间
func (r *Redis) SetEXP(key string, time int64) (bool, error) {
	// 设置过期时间 秒
	n, err := r.rc.Do("EXPIRE", key, time)
	if err != nil {
		fmt.Println(err)
		return false, err
	}
	if n == int64(1) {
		fmt.Println("EXPIRE Success")
		return true, nil
	}
	return false, nil

}

// 关闭连接
func (r *Redis) CloseCon() bool {
	err := r.rc.Close()
	if err != nil {
		//fmt.Println("Close connect error", err)
		return false
	}
	return true
}
func checkTime(time int64) int64 {
	if time <= 0 {
		time = -1
	}
	return time
}
func main() {
	r := Redis{}
	err := r.CreatCon("tcp", "127.0.0.1:6379", "", 1)
	if err != nil {
		fmt.Println("Connect to redis error", err)
		return
	}
	l := list.List{}
	l.PushFront(1)
	l.PushFront(2)
	l.PushBack(2)
	err = r.SetV("12", l)
	if err != nil {
		fmt.Println("redis set failed:", err)
	}
	bb := r.SetNX("1245", 1)
	if !bb {
		fmt.Println("redis set failed:", err)
	} else {
		fmt.Println("set success ")
	}
	err = r.SetVHasTime("12", l, 5)
	if err != nil {
		fmt.Println("redis set failed:", err)
	}
	bb = r.SetNXHasTime("1245", 1, 3)
	if !bb {
		fmt.Println("redis set failed:", err)
	} else {
		fmt.Println("set success ")
	}
	b, err := r.GetV("12")
	if err != nil {
		fmt.Println("redis get failed:", err)
	}
	s := string(b)
	fmt.Println("get -->", s)
	r.ExitsV("12")
	imap := map[string]string{"username": "666", "phonenumber": "888"}
	js, _ := json.Marshal(imap)
	err = r.SetV("kk", js)
	if err != nil {
		fmt.Println("redis set failed:", err)
	}
	b, err = r.GetV("kk")
	if err != nil {
		fmt.Println("redis get failed:", err)
	}
	var imapGet map[string]interface{}
	json.Unmarshal(b, &imapGet)
	fmt.Println("get -->", imapGet["username"])

	r.SetEXP("kk", 1)
	time.Sleep(time.Second * 2)
	r.DeleteV("kk")
	r.CloseCon()
	//c, err := redis.Dial("tcp", "127.0.0.1:6379",redis.DialDatabase(1),redis.DialPassword("ltbo99lyb"))
	//if err != nil {
	//	fmt.Println("Connect to redis error", err)
	//	return
	//}
	//defer c.Close()
	//
	//err = setGetRedis(err, c)
	//
	//err = deleteRedis(err, c)
	//
	//redisJson(err, c)

}

func setGetRedis(err error, c redis.Conn) error {
	_, err = c.Do("SET", "mykey", "123456")
	if err != nil {
		fmt.Println("redis set failed:", err)
	}
	username, err := redis.String(c.Do("GET", "mykey"))
	if err != nil {
		fmt.Println("redis get failed:", err)
	} else {
		fmt.Printf("Get mykey: %v \n", username)
	}
	is_key_exit, err := redis.Bool(c.Do("EXISTS", "mykey1"))
	if err != nil {
		fmt.Println("error:", err)
	} else {
		fmt.Printf("exists or not: %v \n", is_key_exit)
	}
	_, err = c.Do("SET", "mykey1", "superWang")
	if err != nil {
		fmt.Println("redis set failed:", err)
	}
	is_key_exit2, err := redis.Bool(c.Do("EXISTS", "mykey1"))
	if err != nil {
		fmt.Println("error:", err)
	} else {
		fmt.Printf("exists or not: %v \n", is_key_exit2)
	}
	return err
}

func deleteRedis(err error, c redis.Conn) error {
	_, err = c.Do("DEL", "mykey1")
	if err != nil {
		fmt.Println("redis delelte failed:", err)
	}
	is_key_exit3, err := redis.Bool(c.Do("EXISTS", "mykey1"))
	if err != nil {
		fmt.Println("error:", err)
	} else {
		fmt.Printf("exists or not: %v \n", is_key_exit3)
	}
	return err
}

func redisJson(err error, c redis.Conn) {
	key := "profile"
	imap := map[string]string{"username": "666", "phonenumber": "888"}
	value, _ := json.Marshal(imap)
	n, err := c.Do("SETNX", key, value)
	if err != nil {
		fmt.Println(err)
	}
	if n == int64(1) {
		fmt.Println("success")
	}
	var imapGet map[string]string
	valueGet, err := redis.Bytes(c.Do("GET", key))
	if err != nil {
		fmt.Println(err)
	}
	errShal := json.Unmarshal(valueGet, &imapGet)
	if errShal != nil {
		fmt.Println(err)
	}
	fmt.Println(imapGet["username"])
	fmt.Println(imapGet["phonenumber"])
}
