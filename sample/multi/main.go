package main

import (
	manager "github.com/SAIKAII/go-conn-manager"
	"github.com/SAIKAII/go-conn-manager/sample/util"
	"log"
	"net"
	"time"
)

const (
	Connection_Nums = 100

	Write_Nums = 10
)

// 这个主要测试多个连接的通信运作是否正常
func main() {
	manager.InitPackage(2, 512, 512)
	cChan := make([]*util.Codec, Connection_Nums)
	for i := 0; i < Connection_Nums; i++ {
		conn, err := net.Dial("tcp", "localhost:8081")
		if err != nil {
			panic(err)
		}
		codec := util.NewCodec(conn)
		cChan[i] = codec
		go handleConnect(codec)
	}

	for i := 0; i < Write_Nums; i++ {
		for _, c := range cChan {
			err := c.Write(c.Encode([]byte("Time: " + time.Now().String())))
			if err != nil {
				log.Println(err)
			}
		}
	}

	mark := false
	for !mark {
		mark = true
		for _, c := range cChan {
			if !c.IsClosed() || !c.IsEmpty() {
				mark = false
			}
		}
		time.Sleep(1 * time.Second)
	}
}

func handleConnect(codec *util.Codec) {

	for {
		_, err := codec.Read()
		if err != nil {
			log.Println(err)
		}

		b, _, err := codec.Decode()
		if err != nil {
			log.Println(err)
		}

		log.Println(string(b))

		if codec.IsEmpty() && codec.IsClosed() {
			break
		}
	}
}
