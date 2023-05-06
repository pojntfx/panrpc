package main

import (
	"context"
	"flag"
	"log"
	"net"
	"time"

	"github.com/pojntfx/dudirekta/pkg/rpc"
)

type local struct {
	Peers func() map[string]remote
}

func (s *local) Iterate(
	ctx context.Context,
	length int,
	onIteration func(ctx context.Context, i int) error,
) (int, error) {
	for i := 0; i < length; i++ {
		if err := onIteration(ctx, i); err != nil {
			return -1, err
		}
	}

	return length, nil
}

type remote struct{}

func main() {
	addr := flag.String("addr", ":1337", "Listen address")

	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service := &local{}

	clients := 0
	registry := rpc.NewRegistry(
		service,
		remote{},

		time.Second*10,
		ctx,
		&rpc.Options{
			ResponseBufferLen: rpc.DefaultResponseBufferLen,
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
	service.Peers = registry.Peers

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

				if err := registry.Link(conn); err != nil {
					panic(err)
				}
			}()
		}()
	}
}
