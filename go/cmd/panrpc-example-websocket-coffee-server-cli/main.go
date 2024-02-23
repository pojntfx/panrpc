package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"net/http"
	"slices"
	"time"

	"github.com/pojntfx/panrpc/go/pkg/rpc"
	"nhooyr.io/websocket"
)

type coffeeMachine struct {
	supportedVariants []string
	waterLevel        int
}

func (s *coffeeMachine) BrewCoffee(
	ctx context.Context,
	variant string,
	size int,
	onProgress func(ctx context.Context, percentage int) error,
) (int, error) {
	if !slices.Contains(s.supportedVariants, variant) {
		return 0, errors.New("unsupported variant")
	}

	if s.waterLevel-size < 0 {
		return 0, errors.New("not enough water")
	}

	log.Println("Brewing coffee variant", variant, "in size", size, "ml")

	if err := onProgress(ctx, 0); err != nil {
		return 0, err
	}

	time.Sleep(500 * time.Millisecond)
	if err := onProgress(ctx, 25); err != nil {
		return 0, err
	}

	time.Sleep(500 * time.Millisecond)
	if err := onProgress(ctx, 50); err != nil {
		return 0, err
	}

	time.Sleep(500 * time.Millisecond)
	if err := onProgress(ctx, 75); err != nil {
		return 0, err
	}

	time.Sleep(500 * time.Millisecond)
	if err := onProgress(ctx, 100); err != nil {
		return 0, err
	}

	s.waterLevel -= size

	return s.waterLevel, nil
}

type remoteControl struct {
	SetCoffeeMachineBrewing func(ctx context.Context, brewing bool) error
}

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clients := 0

	registry := rpc.NewRegistry[remoteControl, json.RawMessage](
		&coffeeMachine{
			supportedVariants: []string{"latte", "americano"},
			waterLevel:        1000,
		},

		ctx,

		&rpc.Options{
			OnClientConnect: func(remoteID string) {
				clients++

				log.Printf("%v remote controls connected", clients)
			},
			OnClientDisconnect: func(remoteID string) {
				clients--

				log.Printf("%v remote controls connected", clients)
			},
		},
	)

	lis, err := net.Listen("tcp", "127.0.0.1:1337")
	if err != nil {
		panic(err)
	}
	defer lis.Close()

	log.Println("Listening on", lis.Addr())

	if err := http.Serve(lis, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.WriteHeader(http.StatusInternalServerError)

				log.Printf("Remote control disconnected with error: %v", err)
			}
		}()

		switch r.Method {
		case http.MethodGet:
			c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
				OriginPatterns: []string{"*"},
			})
			if err != nil {
				panic(err)
			}

			pings := time.NewTicker(time.Second / 2)
			defer pings.Stop()

			errs := make(chan error)
			go func() {
				for range pings.C {
					if err := c.Ping(ctx); err != nil {
						errs <- err

						return
					}
				}
			}()

			conn := websocket.NetConn(ctx, c, websocket.MessageText)
			defer conn.Close()

			encoder := json.NewEncoder(conn)
			decoder := json.NewDecoder(conn)

			go func() {
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
					errs <- err

					return
				}
			}()

			if err := <-errs; err != nil {
				panic(err)
			}
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})); err != nil {
		panic(err)
	}
}
