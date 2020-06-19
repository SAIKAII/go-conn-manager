package go_conn_manager

import (
	"net"
	"sync"
	"syscall"
	"time"
)

type Conn struct {
	mu       sync.Mutex
	fd       int
	SockAddr syscall.Sockaddr
	data     interface{}
	lastTime int64 // 该套接字最后一次通信时间
}

func (c *Conn) UpdateLastTime() {
	c.lastTime = time.Now().Unix()
}

func (c *Conn) LastTime() int64 {
	return c.lastTime
}

func (c *Conn) Close() {
	syscall.Close(c.fd)
}

func (c *Conn) Fd() int {
	return c.fd
}

func (c *Conn) Addr() net.Addr {
	switch sa := c.SockAddr.(type) {
	case *syscall.SockaddrInet4:
		return &net.IPAddr{IP: sa.Addr[0:]}
	case *syscall.SockaddrInet6:
		return &net.IPAddr{IP: sa.Addr[0:]}
	}

	return nil
}

func (c *Conn) Port() int {
	if sa, ok := c.SockAddr.(*syscall.SockaddrInet4); ok {
		return sa.Port
	} else if sa, ok := c.SockAddr.(*syscall.SockaddrInet6); ok {
		return sa.Port
	}

	return 0
}

func (c *Conn) Data() interface{} {
	return c.data
}

func (c *Conn) SetData(d interface{}) {
	c.data = d
}
