package main

import (
	"context"
	"encoding/json"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"github.com/pojntfx/panrpc/go/pkg/rpc"
)

type local struct {
	counter int64
}

func (s *local) Increment(ctx context.Context, delta int64) (int64, error) {
	log.Println("Incrementing counter by", delta, "for remote with ID", rpc.GetRemoteID(ctx))

	return atomic.AddInt64(&s.counter, delta), nil
}

type remote struct {
	Println func(ctx context.Context, msg string) error
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var clients atomic.Int64
	registry := rpc.NewRegistry[remote, json.RawMessage](
		&local{},

		ctx,

		&rpc.RegistryHooks{
			OnClientConnect: func(remoteID string) {
				log.Printf("%v clients connected", clients.Add(1))
			},
			OnClientDisconnect: func(remoteID string) {
				log.Printf("%v clients connected", clients.Add(-1))
			},
		},
	)

	log.Printf(`Run one of the following commands to run a function on the remote(s):

- kill -SIGHUP %v: Print "Hello, world!"`, os.Getpid())

	go func() {
		done := make(chan os.Signal, 1)
		signal.Notify(done, syscall.SIGHUP)

		<-done

		if err := registry.ForRemotes(func(remoteID string, remote remote) error {
			log.Println("Calling functions for remote with ID", remoteID)

			if err := remote.Println(ctx, "Hello, world!"); err != nil {
				log.Println("Got error for Println func:", err)

				return nil
			}

			return nil
		}); err != nil {
			panic(err)
		}
	}()

	log.Println("Connected to stdin and stdout")

	encoder := json.NewEncoder(os.Stdout)
	decoder := json.NewDecoder(os.Stdin)

	if err := registry.LinkStream(
		func(v rpc.Message[json.RawMessage]) error {
			return encoder.Encode(v)
		},
		func(v *rpc.Message[json.RawMessage]) error {
			return decoder.Decode(v)
		},

		func(v any) (json.RawMessage, error) {
			b, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}

			return json.RawMessage(b), nil
		},
		func(data json.RawMessage, v any) error {
			return json.Unmarshal([]byte(data), v)
		},

		nil,
	); err != nil {
		panic(err)
	}
}
