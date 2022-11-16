package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/mail"
	"time"

	"github.com/pojntfx/dudirekta/pkg/mockup"
)

type local[P any] struct{}

func (s local[P]) Multiply(ctx context.Context, multiplicant, multiplier float64) (product float64) {
	return multiplicant * multiplier
}

func (s local[P]) PrintString(ctx context.Context, msg string) {
	fmt.Println(msg)
}

func (s local[P]) ValidateEmail(ctx context.Context, email string) error {
	_, err := mail.ParseAddress(email)

	return err
}

func (s local[P]) ParseJSON(ctx context.Context, p []byte) (any, error) {
	var output any
	if err := json.Unmarshal(p, &output); err != nil {
		return nil, err
	}

	return output, nil
}

type remote struct {
	Divide func(ctx context.Context, divident, divisor float64) (quotient float64, err error)
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
			log.Println("Calling functions for peer with ID", peerID)

			quotient, err := peer.Divide(ctx, 50, 2)
			if err != nil {
				log.Println("Got division error:", err)
			} else {
				fmt.Println(quotient)
			}

			quotient, err = peer.Divide(ctx, 50, 0)
			if err != nil {
				log.Println("Got division error:", err)
			} else {
				fmt.Println(quotient)
			}
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
