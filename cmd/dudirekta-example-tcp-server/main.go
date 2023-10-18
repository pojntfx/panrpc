package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net"
	"sync/atomic"
	"time"

	"github.com/pojntfx/dudirekta/pkg/rpc"
)

type local struct {
	counter int64
}

func (s *local) Increment(ctx context.Context, delta int64) (int64, error) {
	log.Println("Incrementing counter by", delta, "for peer with ID", rpc.GetRemoteID(ctx))

	return atomic.AddInt64(&s.counter, delta), nil
}

type remote struct {
	Println func(ctx context.Context, msg string) error
}

func main() {
	addr := flag.String("addr", "localhost:1337", "Listen or remote address")
	listen := flag.Bool("listen", true, "Whether to allow connecting to peers by listening or dialing")

	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clients := 0

	registry := rpc.NewRegistry(
		&local{},
		remote{},

		time.Second*10,
		ctx,
		&rpc.Options{
			OnClientConnect: func(remoteID string) {
				clients++

				log.Printf("%v clients connected", clients)
			},
			OnClientDisconnect: func(remoteID string) {
				clients--

				log.Printf("%v clients connected", clients)
			},
		},
	)

	go func() {
		for {
			for peerID, peer := range registry.Peers() {
				log.Println("Calling functions for peer with ID", peerID)

				if err := peer.Println(ctx, "Hello, world!"); err != nil {
					log.Println("Got error for Println func:", err)

					continue
				}
			}
		}
	}()

	if *listen {
		lis, err := net.Listen("tcp", *addr)
		if err != nil {
			panic(err)
		}
		defer lis.Close()

		log.Println("Listening on", lis.Addr())

		for {
			func() {
				conn, err := lis.Accept()
				if err != nil {
					log.Println("could not accept connection, continuing:", err)

					return
				}

				go func() {
					defer func() {
						_ = conn.Close()

						if err := recover(); err != nil {
							log.Printf("Client disconnected with error: %v", err)
						}
					}()

					if err := registry.LinkStream(
						json.NewEncoder(conn).Encode,
						json.NewDecoder(conn).Decode,

						json.Marshal,
						json.Unmarshal,
					); err != nil {
						panic(err)
					}
				}()
			}()
		}
	} else {
		conn, err := net.Dial("tcp", *addr)
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		log.Println("Connected to", conn.RemoteAddr())

		if err := registry.LinkStream(
			json.NewEncoder(conn).Encode,
			json.NewDecoder(conn).Decode,

			json.Marshal,
			json.Unmarshal,
		); err != nil {
			panic(err)
		}
	}
}
