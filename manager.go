package go_conn_manager

import "sync"

type ConnManager struct {
	mu    sync.RWMutex
	conns map[int]*Conn
}

// NewConnManager 生成一个实例
func NewConnManager() *ConnManager {
	return &ConnManager{
		conns: make(map[int]*Conn),
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
