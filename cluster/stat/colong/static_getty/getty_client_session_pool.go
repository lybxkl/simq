package static_getty

import (
	getty "github.com/apache/dubbo-getty"
	"go.uber.org/atomic"
	"time"
)

type Pool struct {
	sessions       chan getty.Session
	maxSize        int
	close          *atomic.Bool
	retrySleepTime int // 获取不到连接每次睡眠时间,ms
}

// NewPool 新建连接池管理
// @maxSize 预计最大大小
// @retrySleepTime 获取不到连接每次睡眠时间,ms
func NewPool(maxSize int, retrySleepTime int) *Pool {
	return &Pool{
		sessions:       make(chan getty.Session, maxSize),
		maxSize:        maxSize,
		retrySleepTime: retrySleepTime,
		close:          atomic.NewBool(false),
	}
}
func (p *Pool) Put(s getty.Session) bool {
	if p.close.Load() {
		return false
	}
	p.sessions <- s
	return true
}
func (p *Pool) Close() {
	p.close.Store(true)
	close(p.sessions)
	for session := range p.sessions {
		session.Close()
	}
}

// Pop TODO 将bool改为error
func (p *Pool) Pop() (getty.Session, bool) {
	if p.close.Load() {
		return nil, false
	}
PP:
	select {
	case s := <-p.sessions:
		if s.IsClosed() {
			goto PP
		}
		return s, true
	default:
		if p.close.Load() {
			return nil, false
		}
		time.Sleep(time.Duration(p.retrySleepTime) * time.Millisecond)
		goto PP
	}
}
