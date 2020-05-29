package main

import (
	"github.com/SAIKAII/go-conn-manager/sample/util"
	"log"
	"net"
	"strconv"
)

func main() {
	conn, err := net.Dial("tcp", "localhost:8081")
	if err != nil {
		panic(err)
	}

	codec := util.NewCodec(conn)

	go func() {
		for {
			_, err := codec.Read()
			if err != nil {
				panic(err)
			}

			b, _, err := codec.Decode()
			if err != nil {
				log.Println(err)
				panic(err)
			}

			log.Println(string(b))
		}
	}()

	for i := 0; i < 10; i++ {
		err := codec.Write(codec.Encode([]byte("This is " + strconv.Itoa(i))))
		if err != nil {
			panic(err)
		}

		log.Println("Write:", i)
	}
	select {}
}
