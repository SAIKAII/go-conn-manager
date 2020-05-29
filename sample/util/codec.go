package util

import (
	"errors"
	packet "github.com/SAIKAII/go-conn-manager"
	"net"
)

type Codec struct {
	conn      *net.TCPConn
	buffer    []byte
	bufferEnd int
}

func NewCodec(c net.Conn) *Codec {
	return &Codec{
		conn:   c.(*net.TCPConn),
		buffer: make([]byte, 65535),
	}
}

func (c *Codec) Read() (int, error) {
	n, err := c.conn.Read(c.buffer[c.bufferEnd:])
	if err != nil {
		return 0, err
	}

	c.bufferEnd += n

	return n, nil
}

func (c *Codec) Decode() ([]byte, int, error) {
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
