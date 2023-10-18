package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"net"
	"strings"
	"sync/atomic"
	"time"

	"github.com/pojntfx/dudirekta/pkg/rpc"
	"github.com/redis/go-redis/v9"
)

const (
	errBusyGroup = "BUSYGROUP Consumer Group name already exists"
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
	redisURL := flag.String("redis-url", "redis://localhost:6379/0", "Redis URL")
	clientRequestsStream := flag.String("client-requests-stream", "/conn1/requests/client", "Redis stream to write requests to client to")
	clientResponseStream := flag.String("client-response-stream", "/conn1/responses/client", "Redis stream to write responses to client to")

	// serverRequestsStream := flag.String("server-requests-stream", "/conn1/requests/server", "Redis stream to read requests from client from")

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

	options, err := redis.ParseURL(*redisURL)
	if err != nil {
		panic(err)
	}

	broker := redis.NewClient(options)
	defer broker.Close()

	if _, err := broker.XGroupCreateMkStream(ctx, *clientRequestsStream, *clientRequestsStream, "$").Result(); err != nil && !strings.Contains(err.Error(), errBusyGroup) {
		panic(err)
	}

	log.Println("Connected to Redis")

	requestPackets := make(chan []byte)
	responsePackets := make(chan []byte)

	// TODO: Connect `broker.XReadGroup`

	if err := registry.LinkMessage(
		func(b []byte) error {
			if _, err := broker.XAdd(ctx, &redis.XAddArgs{
				Stream: *clientRequestsStream,
				Values: map[string]interface{}{
					"packet": b,
				},
			}).Result(); err != nil {
				return err
			}

			return nil
		},
		func(b []byte) error {
			if _, err := broker.XAdd(ctx, &redis.XAddArgs{
				Stream: *clientResponseStream,
				Values: map[string]interface{}{
					"packet": b,
				},
			}).Result(); err != nil {
				return err
			}

			return nil
		},

		func() ([]byte, error) {
			packet, ok := <-requestPackets
			if !ok {
				return []byte{}, net.ErrClosed
			}

			return packet, nil
		},
		func() ([]byte, error) {
			packet, ok := <-responsePackets
			if !ok {
				return []byte{}, net.ErrClosed
			}

			return packet, nil
		},

		json.Marshal,
		json.Unmarshal,
	); err != nil {
		panic(err)
	}
}
