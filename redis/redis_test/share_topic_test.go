package redis_test

import (
	"container/list"
	"fmt"
	"github.com/go-redis/redis"
	"strings"
	"testing"
)

var shareJoin = "/"
var tp = `
--[ i 记录最后一个的索引，用来最后添加sharename集合使用 ]--
	local i = 1
--[ cc 保存shareName  ]--
	local cc = {}
--[ ret保存每一个sharename下的集群订阅数据，最后一个集合是sharename的集合 ]--
	local ret = {}
	for ii,vk in ipairs(KEYS) do
		--[ 获取sharename成员 --]
		local shareName = redis.call("smembers",vk)
		if shareName ~= nil and #shareName > 0
		then
		--[ 遍历sharename，拼接KEYS[1] 获取该共享组下的信息--]
			for k, v in ipairs(shareName) do
				--[ aa 保存当前sharename下的集合数据 ]--
				local aa = {}
				cc[i] = v
    			local ks = vk.."` + shareJoin + `"..v
            	local c = redis.call("hgetall",ks)
				if c ~= nil and #c > 0
				then
					for k2, v2 in ipairs(c) do
						aa[k2] = v2
					end
					ret[i] = aa
					i = i +1
				end
			end
		end
	end
	ret[i] = cc
	return ret
`

func TestTopic(t *testing.T) {
	r := redis.NewClient(&redis.Options{DB: int(1), Password: "", Addr: "10.112.26.131:6379", Network: "tcp"})
	_, err := r.Ping().Result()
	if err != nil {
		panic(err)
	}
	v, err := redis.NewScript(tp).Run(r, matchTopicS("test/pass/123")).Result()
	if err != nil {
		panic(fmt.Errorf("脚本执行错误：%v", err))
	}
	fmt.Println(v)
}

func TestShare(t *testing.T) {
	fmt.Println(matchTopicS("/as/aa/aa"))
	fmt.Println(matchTopicS("/as/aa"))
	fmt.Println(matchTopicS("as/aa/"))
	fmt.Println(matchTopicS("/"))
	fmt.Println(matchTopicS(""))
}
func BenchmarkShare(b *testing.B) {
	for i := 0; i < b.N; i++ {
		matchTopicS("/asdss66666666666666666666666666666666666666dsds/aasdddsaa/aasadas/")
	}
}

// 获取需要匹配的主题
// 下面可优化的地方太多了，性能不是很好
func matchTopicS(topic string) []string {
	tp := strings.Split(topic, "/")
	ret := list.New()
	ret.PushBack(tp[0])
	// 直接限制订阅主题第一个不能是通配符,并且不能是单纯一个/,所以该方法就不做限制
	for k := range tp {
		if k == 0 {
			continue
		}
		v := tp[k]
		size := ret.Len()
		for i := 0; i < size; i++ {
			el := ret.Front()
			s := el.Value.(string)
			if s != "" && s[len(s)-1] == '#' {
				ret.MoveToBack(el)
				continue
			}
			el.Value = s + "/" + v
			ret.MoveToBack(el)
			ret.PushBack(s + "/+")
			ret.PushBack(s + "/#")
		}
	}

	da := make([]string, 0)
	for elem := ret.Front(); elem != nil; elem = elem.Next() {
		vs := elem.Value.(string)
		if vs == "" {
			continue
		}
		da = append(da, vs)
	}
	return da
}
