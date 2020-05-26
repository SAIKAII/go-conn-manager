package go_conn_manager

import (
	"golang.org/x/sys/unix"
	"io"
	"strconv"
	"strings"
	"sync"
	"syscall"
	"time"
)

const (
	Poll_Event_Listen = unix.POLLIN | unix.POLLPRI
	Poll_Event_Read   = unix.POLLIN | unix.POLLPRI | unix.POLLHUP | unix.POLLRDHUP | unix.POLLERR
)

type Poll struct {
	mu       sync.Mutex
	listenFd int
	handler  Handler
	conns    *ConnManager
	fds      map[int32]*unix.PollFd
	revents  chan event
	timeout  time.Duration
	stop     chan struct{}
}

func NewPoll() *Poll {
	return &Poll{
		conns:   NewConnManager(),
		fds:     make(map[int32]*unix.PollFd),
		revents: make(chan event),
		stop:    make(chan struct{}),
	}
}

func (p *Poll) SetHandler(h Handler) {
	p.handler = h
}

func (p *Poll) Init(ipAddr string, port int) error {
	listenFd, err := syscall.Socket(syscall.AF_INET, syscall.SOCK_STREAM, 0)
	if err != nil {
		return err
	}
	p.listenFd = listenFd

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
	err = syscall.Bind(p.listenFd, &syscall.SockaddrInet4{
		Port: port,
		Addr: addr,
	})
	if err != nil {
		return err
	}

	err = syscall.Listen(p.listenFd, Listen_Queue_Size)
	if err != nil {
		return err
	}

	p.fds[int32(listenFd)] = &unix.PollFd{
		Fd:     int32(listenFd),
		Events: Poll_Event_Listen,
	}

	return nil
}

func (p *Poll) WaitEvent() {
	for {
		fds := make([]unix.PollFd, len(p.fds))
		i := 0
		p.mu.Lock()
		for _, val := range p.fds {
			fds[i] = *val
			i++
		}
		p.mu.Unlock()

		select {
		case <-p.stop:
			break
		default:
			n, err := unix.Poll(fds, -1)
			if err != nil {
				continue
			}
			p.handleFds(fds, n)
		}
	}

}

func (p *Poll) handleFds(fds []unix.PollFd, n int) {
	fdCh := make(chan event, n)
	for i := 0; i < n; i++ {
		if fds[i].Revents == 0 {
			continue
		}

		if (fds[i].Revents & unix.POLLIN) > 0 {
			if fds[i].Fd == int32(p.listenFd) {
				fdCh <- event{
					fd:    fds[i].Fd,
					event: Event_Type_Connect,
				}
			} else {
				p.revents <- event{
					fd:    fds[i].Fd,
					event: Event_Type_In,
				}
			}
		} else if (fds[i].Revents & unix.POLLERR) > 0 {
			p.revents <- event{
				fd:    fds[i].Fd,
				event: Event_Type_Error,
			}
		} else if (fds[i].Revents&unix.POLLRDHUP) > 0 || (fds[i].Revents&unix.POLLHUP) > 0 {
			// POLLHUP: FIN has been received and sent.
			fdCh <- event{
				fd:    fds[i].Fd,
				event: Event_Type_Close,
			}
		}
	}
	close(fdCh)
	p.handleConnect(fdCh)
}

func (p *Poll) AddRead(nfd int, c *Conn) error {
	p.fds[int32(nfd)] = &unix.PollFd{
		Fd:     int32(nfd),
		Events: Poll_Event_Read,
	}
	p.conns.AddConn(nfd, c)
	p.handler.OnConnect(c)

	return nil
}

func (p *Poll) Del(nfd int) error {
	delete(p.fds, int32(nfd))
	p.handler.OnClose(p.conns.GetConn(nfd))
	p.conns.DelConn(nfd)

	return nil
}

// handleConnect 处理新增连接与连接关闭
func (p *Poll) handleConnect(fdCh <-chan event) {
	// 该方法不需要加锁是因为这是WaitEvent的同步操作
	for ev := range fdCh {
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
			err = p.AddRead(nfd, c)
			if err != nil {
				continue
			}
		} else if ev.event == Event_Type_Close {
			p.Del(int(ev.fd))
		}
	}
}

func (p *Poll) HandleEvent() error {
	for ev := range p.revents {
		if ev.event == Event_Type_In {
			err := UnpackFromFD(p.conns.GetConn(int(ev.fd)), p.handler.OnMessage)
			if err != nil && err == io.EOF {
				// 读取中检测到对方关闭了套接字
				p.revents <- event{
					fd:    ev.fd,
					event: Event_Type_Close,
				}
			}
		} else if ev.event == Event_Type_Error {
			// In TCP, this typically means a RST has been received or sent.
			p.handler.OnError(p.conns.GetConn(int(ev.fd)))
			p.conns.DelConn(int(ev.fd))
			// 这里的删除需要加锁是因为HandleEvent与WaitEvent是并发运行的
			p.mu.Lock()
			delete(p.fds, ev.fd)
			p.mu.Unlock()
		} else if ev.event == Event_Type_Close {
			// 这里的删除需要加锁是因为HandleEvent与WaitEvent是并发运行的
			p.mu.Lock()
			p.Del(int(ev.fd))
			p.mu.Unlock()
		}
	}
	return nil
}
