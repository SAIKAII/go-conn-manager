package go_conn_manager

import (
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	Listen_Queue_Size = 1024
	Epoll_Create_Size = 1

	Epoll_CTL_Listener = syscall.EPOLLIN | syscall.EPOLLET | syscall.EPOLLPRI
	Epoll_CTL_Read     = syscall.EPOLLIN | syscall.EPOLLET | syscall.EPOLLPRI
)

type Epoll struct {
	epollFd  int
	listenFd int
	revents  chan event
	conns    *ConnManager
	handler  Handler
	timeout  time.Duration
	stop     chan struct{}
}

func NewEpoll() *Epoll {
	return &Epoll{
		revents: make(chan event, 1024),
		conns:   NewConnManager(),
		stop:    make(chan struct{}),
	}
}

func (e *Epoll) SetHandler(h Handler) {
	e.handler = h
}

// 创建一个Epoll实例
func (e *Epoll) Init(ipAddr string, port int) error {
	// Specifying  a  protocol  of  0  causes Socket() to use an unspecified
	// default protocol appropriate for the requested socket type.
	listenFd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		return err
	}
	e.listenFd = listenFd

	addr := [4]byte{}
	if ipAddr != "" {
		s := strings.Split(ipAddr, ".")
		for i := range s {
			v, err := strconv.Atoi(s[i])
			if err != nil {
				return err
			}
			addr[i] = byte(v)
		}
	}
	err = syscall.Bind(e.listenFd, &syscall.SockaddrInet4{
		Port: port,
		Addr: addr,
	})
	if err != nil {
		return err
	}

	err = syscall.Listen(e.listenFd, Listen_Queue_Size)
	if err != nil {
		return err
	}

	// Since Linux 2.6.8, the size argument is ignored, but must be
	// greater than zero
	epollFd, err := syscall.EpollCreate(Epoll_Create_Size)
	if err != nil {
		return err
	}
	e.epollFd = epollFd

	err = syscall.EpollCtl(epollFd, syscall.EPOLL_CTL_ADD, e.listenFd, &syscall.EpollEvent{
		Events: Epoll_CTL_Listener,
		Fd:     int32(listenFd),
	})
	if err != nil {
		return err
	}

	return nil
}

func (e *Epoll) WaitEvent() {
	for {
		select {
		case <-e.stop:
			close(e.revents)
			return
		default:
			events := make([]syscall.EpollEvent, 100)
			n, err := syscall.EpollWait(e.epollFd, events, -1)
			if err != nil {
				continue
			}

			for i := 0; i < n; i++ {
				if events[i].Fd == int32(e.listenFd) {
					e.revents <- event{
						fd:    events[i].Fd,
						event: Event_Type_Connect,
					}
				} else {
					e.revents <- event{
						fd:    events[i].Fd,
						event: Event_Type_In,
					}
				}
			}
		}

	}
}

func (e *Epoll) HandleEvent() error {
	for ev := range e.revents {
		if ev.event == Event_Type_Connect {
			nfd, sa, err := syscall.Accept(int(ev.fd))
			if err != nil {
				continue
			}
			c := &Conn{
				fd:       nfd,
				SockAddr: sa,
				lastTime: time.Now().Unix(),
			}
			err = e.AddRead(nfd, c)
			if err != nil {
				continue
			}
		} else if ev.event == Event_Type_Close {
			nfd := int(ev.fd)
			err := e.Del(nfd)
			if err != nil {
				continue
			}
		} else if ev.event == Event_Type_In {
			err := UnpackFromFD(e.conns.GetConn(int(ev.fd)), e.handler.OnMessage)
			if err != nil {
				continue
			}
		} else {

		}
	}
	return nil
}

// AddRead 把套接字加入监听，创建conn，并调用OnConnect回调函数
func (e *Epoll) AddRead(nfd int, c *Conn) error {
	err := syscall.EpollCtl(e.epollFd, syscall.EPOLL_CTL_ADD, nfd, &syscall.EpollEvent{
		Events: Epoll_CTL_Read,
		Fd:     int32(nfd),
	})
	if err != nil {
		return err
	}

	e.conns.AddConn(nfd, c)
	e.handler.OnConnect(c)

	return nil
}

// Del 从监听中删除套接字，删除conn，并调用OnClose回调函数
func (e *Epoll) Del(nfd int) error {
	err := syscall.EpollCtl(e.epollFd, syscall.EPOLL_CTL_DEL, nfd, nil)
	if err != nil {
		return err
	}

	e.handler.OnClose(e.conns.GetConn(nfd))
	e.conns.DelConn(nfd)

	return nil
}
