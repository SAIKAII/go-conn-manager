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
	epoll    *Epoll
	lastTime int64 // 该套接字最后一次通信时间
}

func (c *Conn) Read() error {
	c.mu.Lock()
	c.mu.Unlock()

	c.lastTime = time.Now().Unix()
	err := UnpackFromFD(c)
	if err != nil {
		return err
	}

	return nil
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
