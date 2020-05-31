package util

import (
	"errors"
	packet "github.com/SAIKAII/go-conn-manager"
	"net"
	"time"
)

type Codec struct {
	conn      *net.TCPConn
	buffer    []byte
	bufferEnd int
	closed    bool
}

func NewCodec(c net.Conn) *Codec {
	return &Codec{
		conn:   c.(*net.TCPConn),
		buffer: make([]byte, 65535),
	}
}

func (c *Codec) Read() (int, error) {
	if c.closed {
		return c.bufferEnd, nil
	}
	c.conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	n, err := c.conn.Read(c.buffer[c.bufferEnd:])
	if err != nil {
		if e, ok := err.(net.Error); ok && e.Timeout() {
			c.CloseConnection()
			c.closed = true
		}
		if c.bufferEnd == 0 {
			return 0, err
		}

	}

	c.bufferEnd += n

	return n, nil
}

func (c *Codec) Decode() ([]byte, int, error) {
	if c.bufferEnd == 0 {
		return nil, 0, errors.New("缓冲区无数据")
	}
	b := packet.Unpack(c.buffer)
	if b == nil {
		return nil, 0, errors.New("解包失败")
	}

	bLen := len(b)
	copy(c.buffer, c.buffer[packet.PackageHeaderLen+bLen:c.bufferEnd])
	c.bufferEnd -= packet.PackageHeaderLen + bLen

	return b, bLen, nil
}

func (c *Codec) Write(data []byte) error {
	n, err := c.conn.Write(data)
	if n != len(data) {
		return errors.New("向连接写数据不完整")
	} else if err != nil {
		return err
	}

	return nil
}

func (c *Codec) Encode(data []byte) []byte {
	b := packet.Packet(data)
	return b
}

func (c *Codec) CloseConnection() {
	c.conn.Close()
}

func (c *Codec) IsEmpty() bool {
	return c.bufferEnd == 0
}

func (c *Codec) IsClosed() bool {
	return c.closed
}
