package main

import (
	"encoding/json"
	"fmt"
	"github.com/k-si/krpc"
	"github.com/k-si/krpc/codec"
	"log"
	"net"
	"time"
)

func serve() {
	lis, _ := net.Listen("tcp", "127.0.0.1:9999")
	krpc.DefaultServer.Run(lis)
}

func main() {
	go serve()

	time.Sleep(time.Second)
	conn, _ := net.Dial("tcp", "127.0.0.1:9999")
	defer conn.Close()

	cc := codec.NewGobCodec(conn)
	_ = json.NewEncoder(conn).Encode(krpc.DefaultOption)
	time.Sleep(time.Second)

	var err error
	for i := 0; i < 5; i++ {
		head := &codec.Head{
			Method: "sayHello",
			Seq:    uint64(i),
		}
		if err = cc.Write(head, fmt.Sprintf("krpc request argv")); err != nil {
			fmt.Println("cc.Write:", err)
		}

		var h codec.Head
		if err = cc.ReadHead(&h); err != nil {
			fmt.Println("cc.ReadHead", err)
		}
		var s string
		if err = cc.ReadBody(&s); err != nil {
			fmt.Println("cc.ReadBody", err)
		}
		log.Println("reply:", s)
	}
}
