package redis

import (
	"container/list"
	"fmt"
	"github.com/go-redis/redis"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	r            *redis.Client
	cacheGlobal  *cacheInfo
	cacheOutTime = 1 * time.Second
	open         bool
	shareJoin    = "/"
)

type tn struct {
	v *ShareNameInfo
	*time.Timer
}
type global map[string]*tn
type cacheInfo struct {
	sync.RWMutex
	global
}

func init() {
	// todo
	r = redis.NewClient(&redis.Options{DB: int(0), Password: "", Addr: "", Network: "tcp"})
	_, err := r.Ping().Result()
	if err != nil {
		panic(err)
	}
	cacheGlobal = &cacheInfo{
		RWMutex: sync.RWMutex{},
		global:  make(map[string]*tn),
	}
	cacheGlobal.autoClea()
}

// 定时清理缓存
func (cg *cacheInfo) autoClea() {
	go func() {
		c := time.Tick(cacheOutTime)
		for _ = range c {
			cacheGlobal.Lock()
			for k, v := range cacheGlobal.global {
				select {
				case <-v.C:
					delete(cacheGlobal.global, k)
				default:
					continue
				}
			}
			cacheGlobal.Unlock()
		}
	}()
}
func isEmpty(str ...string) bool {
	for _, v := range str {
		if v == "" {
			return true
		}
	}
	return false
}

// 获取数据脚本
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

// 更新脚本
var sr = `
  redis.call("sadd",KEYS[1],KEYS[2])
  local k = KEYS[1].."` + shareJoin + `"..KEYS[2]
  local value = redis.call("hget",k,KEYS[3])
  if value == nil
   then
    return redis.call("hset",k,KEYS[3],ARGV[1])
  else 
    local p = tonumber(value)
    if p == nil
    then 
      return redis.call("hset",k,KEYS[3],ARGV[1])
    else
      if p < 0
      then return redis.call("hincrby",k,KEYS[3],-1 * p)
      else 
        if p + ARGV[2] < 0
        then return redis.call("hincrby",k,KEYS[3],0)
        else return redis.call("hincrby",k,KEYS[3],ARGV[2])
        end
      end
    end
  end
`

// 删除主题脚本
var del = `
	local shareName = redis.call("smembers",KEYS[1])
	if shareName ~= nil
	then
		for k, v in ipairs(shareName) do
            redis.call("del",KEYS[1].."` + shareJoin + `"..v)
		end
		return redis.call("del",KEYS[1])
	end
	return 0
`

// 删除节点相关订阅数据脚本
var delNode = `
	for i, v in ipairs(KEYS) do
		redis.call("hdel",v,ARGV[1])
	end
	return 1
`
var merge = &Group{}

func reqMerge(topic string) (*ShareNameInfo, error) {
	ret := merge.DoChan(topic, func() (interface{}, error) {
		v, err := redis.NewScript(tp).Run(r, matchTopicS(topic)).Result()
		if err != nil {
			return nil, fmt.Errorf("脚本执行错误：%v", err)
		}
		if v1, ok := v.([]interface{}); ok {
			ln := len(v1)
			if ln == 0 || ln == 1 {
				return nil, fmt.Errorf("未查询到数据")
			}
			name := v1[ln-1].([]interface{})
			retdata := make([][]interface{}, 0)
			for i := 0; i < ln-1; i++ {
				retdata = append(retdata, v1[i].([]interface{}))
			}
			sni := NewShareNameInfo()
			for i, da := range name {
				tv := 0
				mp := make(map[string]int)
				for j := 0; j < len(retdata[i]); j += 2 {
					ii, err := strconv.Atoi(retdata[i][j+1].(string))
					if err != nil {
						return nil, err
					}
					mp[retdata[i][j].(string)] = ii
					tv += ii
				}
				if mv, ok := sni.V[da.(string)]; ok {
					for nod, d := range mp {
						mv[nod] += d
					}
				} else {
					sni.V[da.(string)] = mp
				}
				sni.t[da.(string)] += tv
			}
			cacheGlobal.Lock()
			cacheGlobal.global[topic] = &tn{
				v:     sni,
				Timer: time.NewTimer(cacheOutTime),
			}
			cacheGlobal.Unlock()
			return sni, nil
		}
		return nil, fmt.Errorf("获取数据错误")
	})
	select {
	case p := <-ret:
		if p.Err != nil {
			return &ShareNameInfo{}, p.Err
		}
		return p.Val.(*ShareNameInfo), nil
	case <-time.After(time.Millisecond * 300):
		return nil, fmt.Errorf("请求超时")
	}
}

// 获取topic的共享主题信息
func GetTopicShare(topic string) (*ShareNameInfo, error) {
	if !open {
		return nil, nil
	}
	cacheGlobal.RLock()
	if v, ok := cacheGlobal.global[topic]; ok {
		defer cacheGlobal.RUnlock()
		return v.v, nil
	}
	cacheGlobal.RUnlock()
	return reqMerge(topic)
}

// 新增一个topic下某个shareName的订阅
// A：$share/a_b/c
// B：$share/a/b_c
// 如果采用非/,+,#的拼接符
// 会出现redis中冲突的情况，
// 可以考虑换一个拼接符 '/'，因为$share/{shareName}/{filter} 中shareName中不能出现'/'的
// 上述已修改为 '/'
func SubShare(topic, shareName, nodeName string) bool {
	if !open {
		return true
	}
	v, err := redis.NewScript(sr).Run(r, []string{topic, shareName, nodeName}, 1, 1).Result()
	if err != nil {
		return false
	}
	return v.(int64) > 0
}

// 取消一个topic下某个shareName的订阅
func UnSubShare(topic, shareName, nodeName string) bool {
	if !open {
		return true
	}
	v, err := redis.NewScript(sr).Run(r, []string{topic, shareName, nodeName}, 0, -1).Result()
	if err != nil {
		return false
	}
	return v.(int64) > 0
}

// 删除主题
func DelTopic(topic string) error {
	if !open {
		return nil
	}
	_, err := redis.NewScript(del).Run(r, []string{topic}).Result()
	if err != nil {
		return fmt.Errorf("删除topic：%v下的共享信息出错：%v", topic, err)
	}
	return nil
}

// 删除节点相关订阅数据
// 思路：获取当前节点的share manage，查询哪写需要删除，然后一次性删除
func DelNode(old map[string][]string, nodeName string) error {
	if !open {
		return nil
	}
	pa := make([]string, 0)
	for tpc, v := range old {
		for _, s := range v {
			pa = append(pa, tpc+shareJoin+s)
		}
	}
	_, err := redis.NewScript(delNode).Run(r, pa, []string{nodeName}).Result()
	if err != nil {
		return fmt.Errorf("删除当前节点的所有共享信息出错：%v", err)
	}
	return nil
}

type ShareNameInfo struct {
	sync.RWMutex
	V map[string]map[string]int
	t map[string]int // 每个shareName下的总数
}

func NewShareNameInfo() *ShareNameInfo {
	return &ShareNameInfo{
		V: make(map[string]map[string]int),
		t: make(map[string]int),
	}
}

// 返回不同共享名称组应该发送给哪个节点的数据
// 返回节点名称：节点需要发送的共享组名称
func (s *ShareNameInfo) RandShare() map[string][]string {
	s.RLock()
	defer s.RUnlock()
	ret := make(map[string][]string)
	for shareName, nodes := range s.V {
		if s.t[shareName] <= 0 {
			continue
		}
		randN := rand.New(rand.NewSource(time.Now().UnixNano())).Intn(s.t[shareName])
		for k, v := range nodes {
			randN -= v
			if randN > 0 {
				continue
			}
			ret[k] = append(ret[k], shareName)
		}
	}
	return ret
}

// 获取需要匹配的主题
// FIXME 下面可优化的地方太多了，因为性能不是很好
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
