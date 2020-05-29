package main

import (
	manager "github.com/SAIKAII/go-conn-manager"
	"log"
	"os"
	"os/signal"
	"time"
)

type handler struct {
}

func (*handler) OnConnect(c *manager.Conn) {
	log.Println("OnConnect, FD:", c.Fd())
}

func (*handler) OnMessage(c *manager.Conn, data []byte) {
	log.Println("OnMessage, FD:", c.Fd(), "data:", string(data))
	manager.PacketToPeer(c, data)
}
func (*handler) OnClose(c *manager.Conn) error {
	log.Println("OnClose:", c.Fd())
	return nil
}
func (*handler) OnError(c *manager.Conn) {
	log.Println("OnError:", c.Fd())
}

func main() {
	epoll := manager.NewEpoll(5 * time.Second)
	server := manager.NewServer(epoll)
	go server.Start("127.0.0.1", 8081, 2, 512, 512, &handler{})
	c := make(chan os.Signal)
	signal.Notify(c, os.Interrupt, os.Kill)
	select {
	case <-c:
		server.Stop()
	}
}
