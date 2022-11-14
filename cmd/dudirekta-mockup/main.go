package main

import (
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"time"

	"github.com/pojntfx/dudirekta/pkg/mockup"
)

type local[P any] struct{}

func (s local[P]) Divide(ctx context.Context, divident, divisor float64) (quotient float64, err error) {
	if divisor == 0 {
		return -1, errors.New("could not divide by zero")
	}

	return divident / divisor, nil
}

type remote struct {
	Multiply func(ctx context.Context, multiplicant, multiplier float64) (product float64)
}

func main() {
	addr := flag.String("addr", "localhost:1337", "Listen or remote address")
	listen := flag.Bool("listen", true, "Whether to allow connecting to peers by listening or dialing")

	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := mockup.NewRegistry(
		&local[remote]{},
		remote{},
		ctx,
	)

	time.AfterFunc(time.Second*5, func() {
		for peerID, peer := range registry.Peers() {
			log.Println(peerID, peer.Multiply(ctx, 5, 2))
		}
	})

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
}
