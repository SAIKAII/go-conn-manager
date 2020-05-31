package main

import (
	"fmt"
	manager "github.com/SAIKAII/go-conn-manager"
	"github.com/SAIKAII/go-conn-manager/sample/util"
	"log"
	"net"
	"strconv"
)

func main() {
	manager.InitPackage(2, 512, 512)
	conn, err := net.Dial("tcp", "localhost:8081")
	if err != nil {
		panic(err)
	}

	codec := util.NewCodec(conn)

	stop := make(chan struct{})
	go func() {
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
		log.Println("close")
		close(stop)
	}()

	for i := 0; i < 10; i++ {
		err := codec.Write(codec.Encode([]byte("This is " + strconv.Itoa(i))))
		if err != nil {
			panic(err)
		}

		log.Println("Write:", i)
	}

	select {
	case <-stop:
		fmt.Println("Stop")
	}
}
