package go_conn_manager

import (
	"sync"
	"time"
)

type ConnManager struct {
	mu      sync.RWMutex
	conns   map[int]*Conn
	ticker  *time.Ticker
	elapsed int64
	stop    chan struct{}
}

// NewConnManager 生成一个实例
func NewConnManager(interval time.Duration) *ConnManager {
	return &ConnManager{
		conns:   make(map[int]*Conn),
		ticker:  time.NewTicker(interval),
		elapsed: interval.Milliseconds(),
		stop:    make(chan struct{}),
	}
}

// AddConn 添加指定key的Conn实例到管理器中，若该key已关联Conn实例，则被更换为新的
func (cm *ConnManager) AddConn(key int, conn *Conn) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	// 关闭连接，删除记录
	if c, ok := cm.conns[key]; ok {
		c.Close()
		delete(cm.conns, key)
	}
	cm.conns[key] = conn
}

// DelConn 删除指定key的Conn实例
func (cm *ConnManager) DelConn(key int) {
	cm.mu.Lock()
	defer cm.mu.Unlock()

	if c, ok := cm.conns[key]; ok {
		c.Close()
		delete(cm.conns, key)
	}
}

// GetConn 获取指定key关联的Conn实例
func (cm *ConnManager) GetConn(key int) *Conn {
	return cm.conns[key]
}

// CheckTimeout 把在指定时间内一次通信都没有的连接关闭，
// 因为也许对方由于某些原因已经不使用该连接
func (cm *ConnManager) CheckTimeout() {
	for {
		select {
		case <-cm.ticker.C:
			cm.check()
		case <-cm.stop:
			return
		}
	}
}

func (cm *ConnManager) check() {
	for k, v := range cm.conns {
		interval := time.Now().Unix() - v.LastTime()
		if interval < (cm.elapsed / 1000) {
			continue
		}

		delete(cm.conns, k)
	}
}

// StopCheck 停止检查连接
func (cm *ConnManager) StopCheck() {
	cm.stop <- struct{}{}
}
