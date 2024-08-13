package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"

	"github.com/pojntfx/panrpc/go/pkg/rpc"
)

type local struct{}

func (s *local) Println(ctx context.Context, msg string) error {
	log.Println("Printing message", msg, "for remote with ID", rpc.GetRemoteID(ctx))

	fmt.Println(msg)

	return nil
}

type remote struct {
	Increment func(ctx context.Context, delta int64) (int64, error)
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var clients atomic.Int64
	registry := rpc.NewRegistry[remote, json.RawMessage](
		&local{},

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

- kill -SIGHUP %v: Increment remote counter by one
- kill -SIGUSR2 %v: Decrement remote counter by one`, os.Getpid(), os.Getpid())

	go func() {
		done := make(chan os.Signal, 1)
		signal.Notify(done, syscall.SIGHUP)

		<-done

		if err := registry.ForRemotes(func(remoteID string, remote remote) error {
			log.Println("Calling functions for remote with ID", remoteID)

			new, err := remote.Increment(ctx, 1)
			if err != nil {
				log.Println("Got error for Increment func:", err)

				return nil
			}

			log.Println(new)

			return nil
		}); err != nil {
			panic(err)
		}
	}()

	go func() {
		done := make(chan os.Signal, 1)
		signal.Notify(done, syscall.SIGUSR2)

		<-done

		if err := registry.ForRemotes(func(remoteID string, remote remote) error {
			log.Println("Calling functions for remote with ID", remoteID)

			new, err := remote.Increment(ctx, -1)
			if err != nil {
				log.Println("Got error for Increment func:", err)

				return nil
			}

			log.Println(new)

			return nil
		}); err != nil {
			panic(err)
		}
	}()

	log.Println("Connected to stdin and stdout")

	encoder := json.NewEncoder(os.Stdout)
	decoder := json.NewDecoder(os.Stdin)

	if err := registry.LinkStream(
		ctx,

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
