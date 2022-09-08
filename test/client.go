package main

import (
	"fmt"
	"github.com/k-si/krpc"
	"log"
	"net"
	"sync"
	"time"
)

func serve() {
	lis, _ := net.Listen("tcp", "127.0.0.1:9999")
	krpc.DefaultServer.Run(lis)
}

func main() {
	go serve()

	time.Sleep(time.Second)
	cli, err := krpc.Dial("tcp", "127.0.0.1:9999", krpc.DefaultOption)
	if err != nil {
		log.Fatal(err)
	}
	defer cli.Close()

	time.Sleep(time.Second)

	//var err error
	var wg sync.WaitGroup
	wg.Add(5)
	for i := 0; i < 5; i++ {
		go func(i int) {
			defer wg.Done()
			var reply string
			if err = cli.Call("krpc.method", fmt.Sprintf("rpc req %d", i), &reply); err != nil {
				log.Fatal(err)
			}
			log.Println("server reply:", reply)
		}(i)
	}

	wg.Wait()
}
