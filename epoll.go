package go_conn_manager

import (
	"golang.org/x/sys/unix"
	"io"
	"strconv"
	"strings"
	"syscall"
	"time"
)

const (
	Listen_Queue_Size = 1024
	Epoll_Create_Size = 1

	Epoll_CTL_Listener = syscall.EPOLLIN | unix.EPOLLET | syscall.EPOLLPRI
	Epoll_CTL_Read     = syscall.EPOLLIN | unix.EPOLLET | syscall.EPOLLPRI | syscall.EPOLLRDHUP | syscall.EPOLLHUP | syscall.EPOLLERR
)

type Epoll struct {
	epollFd  int
	listenFd int
	revents  chan event
	conns    *ConnManager
	handler  Handler
	ticker   *time.Ticker
	interval int64
	stop     chan struct{}
}

// NewEpoll 创建Epoll实例，interval指定检测长时间未使用的连接并关闭其
func NewEpoll(interval time.Duration) *Epoll {
	return &Epoll{
		revents:  make(chan event, 1024),
		conns:    NewConnManager(interval),
		ticker:   time.NewTicker(interval),
		interval: int64(interval.Seconds()),
		stop:     make(chan struct{}),
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

	go e.checkTimeout()
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
				if (events[i].Events & syscall.EPOLLIN) > 0 {
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
				} else if (events[i].Events & syscall.EPOLLERR) > 0 {
					e.revents <- event{
						fd:    events[i].Fd,
						event: Event_Type_Error,
					}
				} else if (events[i].Events&syscall.EPOLLRDHUP) > 0 || (events[i].Events&syscall.EPOLLHUP) > 0 {
					// EPOLLHUP: FIN has been received and sent.
					e.revents <- event{
						fd:    events[i].Fd,
						event: Event_Type_Close,
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
			go func(ev event) {
				err := UnpackFromFD(e.conns.GetConn(int(ev.fd)), e.handler.OnMessage)
				if err != nil && err == io.EOF {
					e.revents <- event{
						fd:    int32(ev.fd),
						event: Event_Type_Close,
					}
				}
				e.conns.GetConn(int(ev.fd)).UpdateLastTime()
			}(ev)
		} else if ev.event == Event_Type_Error {
			// In TCP, this typically means a RST has been received or sent.
			e.handler.OnError(e.conns.GetConn(int(ev.fd)))
			e.conns.DelConn(int(ev.fd))
			syscall.EpollCtl(e.epollFd, syscall.EPOLL_CTL_DEL, int(ev.fd), nil)
		}
	}
	return nil
}

func (e *Epoll) Stop() {
	close(e.stop)
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

// checkTimeout 把在指定时间内一次通信都没有的连接关闭，
// 因为也许对方由于某些原因已经不使用该连接
func (e *Epoll) checkTimeout() {
	for {
		select {
		case <-e.ticker.C:
			e.check()
		case <-e.stop:
			break
		}
	}
}

func (e *Epoll) check() {
	conns := e.conns.Conns()
	for k, v := range conns {
		interval := time.Now().Unix() - v.LastTime()
		if interval < e.interval {
			continue
		}

		e.revents <- event{
			fd:    int32(k),
			event: Event_Type_Close,
		}
	}
}
