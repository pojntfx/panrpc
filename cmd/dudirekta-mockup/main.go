package main

import (
	"flag"
	"fmt"
	"log"
	"net"

	"github.com/pojntfx/dudirekta/pkg/mockup"
)

type local[P any] struct {
	Divide func(peer P, divident, divisor float64) (quotient float64, err error)
}

type remote struct {
	Multiply func(multiplicant, multiplier float64) (product float64)
}

func main() {
	addr := flag.String("addr", "localhost:1337", "Listen or remote address")
	listen := flag.Bool("listen", true, "Whether to allow connecting to peers by listening or dialing")

	flag.Parse()

	localStruct := &local[remote]{}
	remoteStruct := &remote{}

	registry := mockup.NewRegistry(localStruct, remoteStruct)

	if *listen {
		lis, err := net.Listen("tcp", *addr)
		if err != nil {
			panic(err)
		}
		defer lis.Close()

		log.Printf("Listening on dudirekta+%v://%v", lis.Addr().Network(), lis.Addr().String())

		if err := registry.Listen(lis); err != nil {
			panic(err)
		}
	} else {
		conn, err := net.Dial("tcp", *addr)
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		log.Printf("Connected to dudirekta+%v://%v", conn.RemoteAddr().Network(), conn.RemoteAddr().String())

		if err := registry.Connect(conn); err != nil {
			panic(err)
		}
	}

	fmt.Println(remoteStruct.Multiply(5, 2))
}
