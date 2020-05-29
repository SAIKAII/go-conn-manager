package go_conn_manager

import (
	"encoding/binary"
	"errors"
	"io"
	"sync"
	"syscall"
)

var (
	PackageHeaderLen   int
	PackageReadMaxLen  int
	PackageWriteMaxLen int
)

type HandleMessage func(*Conn, []byte)

var (
	readBufferPool  *sync.Pool
	writeBufferPool *sync.Pool
)

func InitPackage(headerLen, readMaxLen, writeMaxLen int) {
	PackageHeaderLen = headerLen
	PackageReadMaxLen = readMaxLen
	PackageWriteMaxLen = writeMaxLen

	readBufferPool = &sync.Pool{
		New: func() interface{} {
			return make([]byte, readMaxLen)
		},
	}
	writeBufferPool = &sync.Pool{
		New: func() interface{} {
			return make([]byte, writeMaxLen)
		},
	}
}

// 封包，失败返回nil
func Packet(data []byte) []byte {
	dataLen := len(data)
	retData := make([]byte, PackageHeaderLen+dataLen)
	binary.BigEndian.PutUint16(retData[0:PackageHeaderLen], uint16(dataLen))
	n := copy(retData[PackageHeaderLen:], data)
	if n != dataLen {
		return nil
	}

	return retData
}

// 解包，失败返回nil
func Unpack(data []byte) []byte {
	dataLen := binary.BigEndian.Uint16(data[0:PackageHeaderLen])
	retData := make([]byte, dataLen)
	n := copy(retData, data[PackageHeaderLen:PackageHeaderLen+int(dataLen)])
	if n != int(dataLen) {
		return nil
	}
	return retData
}

// 读取、解包并处理
func UnpackFromFD(c *Conn, h HandleMessage) error {
	readBuffer := readBufferPool.Get()
	defer readBufferPool.Put(readBuffer)

	var byte = readBuffer.([]byte)
	fd := c.Fd()
	for {
		n, _, err := syscall.Recvfrom(fd, byte, syscall.MSG_PEEK|syscall.MSG_DONTWAIT)
		if err != nil {
			// no data is waiting to be received
			if err == syscall.EAGAIN {
				return nil
			}
			return err
		}
		if n == 0 {
			return io.EOF
		}

		if n < PackageHeaderLen {
			return nil
		}

		dataLen := int(binary.BigEndian.Uint16(byte[0:PackageHeaderLen]))
		if dataLen+PackageHeaderLen > n {
			return nil
		}
		n, _, err = syscall.Recvfrom(fd, byte[0:PackageHeaderLen+dataLen], syscall.MSG_DONTWAIT)
		if err != nil {
			return err
		}

		c.UpdateLastTime()
		h(c, byte[PackageHeaderLen:PackageHeaderLen+dataLen])
	}
}

// 封包并发送
func PacketToPeer(c *Conn, data []byte) error {
	dataLen := len(data)
	if dataLen > PackageWriteMaxLen {
		return errors.New("数据超出最大长度限制")
	}

	writeBuffer := writeBufferPool.Get()
	defer writeBufferPool.Put(writeBufferPool)

	var buffer = writeBuffer.([]byte)
	// 写入数据长度
	binary.BigEndian.PutUint16(buffer[0:PackageHeaderLen], uint16(dataLen))
	// 把数据拷贝入发送缓冲区
	n := copy(buffer[PackageHeaderLen:], data)
	if n != dataLen {
		return errors.New("数据拷贝发生错误")
	}

	_, err := syscall.Write(c.Fd(), buffer)
	if err != nil {
		return err
	}

	return nil
}
