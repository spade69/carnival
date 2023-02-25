package main

import (
	"fmt"
	"log"
	"net"
	"sync"
	"time"

	"github.com/spade69/carnival/client"
	"github.com/spade69/carnival/server"
)

func startServer(addr chan string) {
	// pick a free port
	l, err := net.Listen("tcp", ":0")
	if err != nil {
		log.Fatal("network error:", err)

	}
	log.Println("start rpc server on", l.Addr())
	addr <- l.Addr().String()
	server.NewServer().Accept(l)
}

func main() {
	log.SetFlags(0)
	addr := make(chan string)
	go startServer(addr)
	// client rpc
	endPoint, _ := client.Dial("tcp", <-addr)
	defer func() { _ = endPoint.Close() }()

	time.Sleep(time.Second)
	// send options
	// _ = json.NewEncoder(conn).Encode(server.DefaultOption)
	// new gob codec
	// cc := codec.NewGobCodec(conn)
	var wg sync.WaitGroup

	// send request & recive responses
	for i := 0; i < 5; i++ {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			args := fmt.Sprintf("rpc req %d", i)
			var reply string
			if err := endPoint.Call("Foo.Sum", args, &reply); err != nil {
				log.Fatal("call Foo.Sum error:", err)

			}
			log.Println("reply:", reply)

		}(i)
		// h := &codec.Header{
		// 	ServiceMethod: "Foo.Sum",
		// 	Seq:           uint64(i),
		// }
		// _ = cc.Write(h, fmt.Sprintf("rpc req %d", h.Seq))
		// _ = cc.ReadHeader(h)
		// var reply string
		// _ = cc.ReadBody(&reply)
		// log.Println("reply:", reply)

	}
	wg.Wait()
}
