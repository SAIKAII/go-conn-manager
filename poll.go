package go_conn_manager

import (
	"golang.org/x/sys/unix"
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
	for i := 0; i < n; i++ {
		if fds[i].Revents == 0 {
			continue
		}

		if fds[i].Revents == unix.POLLIN {
			if fds[i].Fd == int32(p.listenFd) {
				p.revents <- event{
					fd:    fds[i].Fd,
					event: Event_Type_Connect,
				}
			} else {
				p.revents <- event{
					fd:    fds[i].Fd,
					event: Event_Type_In,
				}
			}
		} else if fds[i].Revents == unix.POLLERR {
			p.revents <- event{
				fd:    fds[i].Fd,
				event: Event_Type_Error,
			}
		} else if fds[i].Revents == unix.POLLRDHUP {
			p.revents <- event{
				fd:    fds[i].Fd,
				event: Event_Type_Close,
			}
		} else if fds[i].Revents == unix.POLLHUP {
			// FIN has been received and sent.
			p.conns.DelConn(int(fds[i].Fd))
			delete(p.fds, fds[i].Fd)
		}
	}
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

func (p *Poll) handleConnect(fd int32) {
	p.mu.Lock()
	e := &unix.PollFd{
		Fd:      fd,
		Events:  Poll_Event_Read,
		Revents: 0,
	}
	p.fds[fd] = e
	p.mu.Unlock()
}

func (p *Poll) HandleEvent() error {
	for ev := range p.revents {
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
		} else if ev.event == Event_Type_In {
			err := UnpackFromFD(p.conns.GetConn(int(ev.fd)), p.handler.OnMessage)
			if err != nil {
				continue
			}
		} else if ev.event == Event_Type_Error {
			// In TCP, this typically means a RST has been received or sent.
			p.handler.OnError(p.conns.GetConn(int(ev.fd)))
			p.conns.DelConn(int(ev.fd))
			delete(p.fds, ev.fd)
		} else if ev.event == Event_Type_Close {
			p.Del(int(ev.fd))
		}
	}
	return nil
}
