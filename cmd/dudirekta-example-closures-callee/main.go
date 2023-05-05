package main

import (
	"context"
	"flag"
	"log"
	"net"
	"time"

	"github.com/pojntfx/dudirekta/pkg/closures"
	"github.com/pojntfx/dudirekta/pkg/rpc"
)

type local struct {
	Peers func() map[string]remote
}

func (s *local) Iterate(
	ctx context.Context,
	length int,
	onIterationClosureID string,
) (int, error) {
	peerID := rpc.GetRemoteID(ctx)

	for i := 0; i < length; i++ {
		for candidateIP, peer := range s.Peers() {
			if candidateIP == peerID {
				if _, err := peer.CallClosure(ctx, onIterationClosureID, []interface{}{i}); err != nil {
					return -1, err
				}
			}
		}
	}

	return length, nil
}

type remote struct {
	CallClosure closures.CallClosureType
}

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
