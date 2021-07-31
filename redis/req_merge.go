package redis

import (
	"errors"
	"fmt"
	"runtime"
	"sync"
)

// 请求合并

type Group struct {
	sync.Mutex
	m map[string]*call
}

// 每个相同请求的回调
type call struct {
	wg       sync.WaitGroup
	val      interface{}     // 函数返回值，只会写入一次，因为只会让第一个创建的去请求结果
	err      error           //请求过程中出现的错误
	released bool            //是否释放
	dups     int             //调用次数
	chans    []chan<- Result // 返回的结果
}
type Result struct {
	Val  interface{}
	Err  error
	Dups int
}

type panicError struct {
	err string
}

func (p panicError) Error() string {
	return p.err
}

var panicE = panicError{"panic：程序运行错误"}

func newPanicError(err interface{}) panicError {
	return panicError{err: fmt.Sprintf("%v: %v", panicE.Error(), err)}
}

type runtimeError struct {
	err string
}

func (p runtimeError) Error() string {
	return p.err
}

var runtimeE = runtimeError{"runtime: 运行时异常"}

func newRuntimeError(err interface{}) runtimeError {
	return runtimeError{err: fmt.Sprintf("%v: %v", runtimeE.Error(), err)}
}

// 同步阻塞，容易因为fn中的阻塞，hang住了整个请求，导致全部都阻塞在这
// 最好使用DoChan，并对返回的chan做超时控制，防止因为单飞的那个请求一直阻塞，导致的全部阻塞
func (g *Group) Do(key string, fn func() (interface{}, error)) (v interface{}, err error, shared bool) {
	g.Lock()
	if g.m == nil { // 懒初始化
		g.m = make(map[string]*call)
	}
	// 查看key是否存在
	if c, ok := g.m[key]; ok {
		// 存在就阻塞等着
		c.dups++
		// 顺便解锁
		g.Unlock()
		// 然后等待
		c.wg.Wait()
		// 区分panic错误和runtime错误
		switch c.err.(type) {
		case panicError:
			panic(c.err)
		case runtimeError:
			runtime.Goexit()
		}
		return c.val, c.err, true
	}
	// 没有这个key 就新创建call
	c := new(call)
	c.wg.Add(1)
	g.m[key] = c
	g.Unlock()
	// 调用call
	g.doCall(c, key, fn)
	return c.val, c.err, c.dups > 0
}

// 使用了两个 defer 巧妙的将 runtime 的错误和我们传入 function 的 panic 区别开来
// 避免了由于传入的 function panic 导致的死锁
func (g *Group) doCall(c *call, key string, fn func() (interface{}, error)) {
	normalReturn := false
	recovered := false
	defer func() {
		// 如果既没有正常执行完毕，又没有 recover 那就说明需要直接退出了
		if !normalReturn && !recovered {
			c.err = newRuntimeError(errors.New("退出"))
		}
		c.wg.Done()
		g.Lock()
		defer g.Unlock()
		// 如果已经released，就不需要重新删除这个key了
		if !c.released {
			delete(g.m, key)
		}
		switch c.err.(type) {
		case panicError:
			if len(c.chans) > 0 {
				go func() { panic(c.err) }()
				select {}
			} else {
				panic(c.err)
			}
		case runtimeError:
			// 准备退出，也就不需要做其他动作
		default:
			// 正常情况，向chan中发送数据
			for _, ch := range c.chans {
				ch <- Result{
					Val:  c.val,
					Err:  c.err,
					Dups: c.dups,
				}
			}
		}
	}()
	func() {
		defer func() {
			if !normalReturn {
				// 如果 panic 了我们就 recover 掉，然后 new 一个 panic 的错误
				// 后面在上层重新 panic
				if r := recover(); r != nil {
					c.err = newPanicError(r)
				}
			}
		}()
		c.val, c.err = fn()
		// 如果fn 没有panic，就会执行到这一步，如果panic了就不会
		normalReturn = true
	}()
	// if normalReturn == false 表示fn panic了
	// 如果执行到了这一步，也说明我们的fn recover 了，不是直接runtime exit
	if !normalReturn {
		recovered = true
	}
}
func (g *Group) DoChan(key string, fn func() (interface{}, error)) <-chan Result {
	ch := make(chan Result, 1)
	g.Lock()
	if g.m == nil {
		g.m = make(map[string]*call)
	}
	if c, ok := g.m[key]; ok {
		c.dups++
		c.chans = append(c.chans, ch)
		g.Unlock()
		return ch
	}
	c := &call{
		wg:    sync.WaitGroup{},
		chans: []chan<- Result{ch},
	}
	c.wg.Add(1)
	g.m[key] = c
	g.Unlock()
	go g.doCall(c, key, fn)
	return ch
}

// 请求成功了，释放
func (g *Group) Release(key string) {
	g.Lock()
	if c, ok := g.m[key]; ok {
		c.released = true
	}
	delete(g.m, key)
	g.Unlock()
}
