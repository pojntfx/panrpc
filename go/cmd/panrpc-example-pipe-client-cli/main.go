package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
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

	clients := 0

	registry := rpc.NewRegistry[remote, json.RawMessage](
		&local{},

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

	log.Printf(`Run one of the following commands to run a function on the remote(s):

- kill -%v %v: Increment remote counter by one
- kill -%v %v: Decrement remote counter by one`,
		1, os.Getpid(),
		5, os.Getpid())

	go func() {
		done := make(chan os.Signal, 1)
		signal.Notify(done, syscall.Signal(1))

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
		signal.Notify(done, syscall.Signal(5))

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
	); err != nil {
		panic(err)
	}
}
