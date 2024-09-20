package main

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net"
	"net/http"
	"slices"
	"sync/atomic"
	"time"

	"github.com/pojntfx/panrpc/go/pkg/rpc"
	"nhooyr.io/websocket"
)

type teaBrewer struct {
	supportedVariants []string
}

func (s *teaBrewer) GetVariants(ctx context.Context) ([]string, error) {
	return s.supportedVariants, nil
}

type coffeeMachine[E any] struct {
	Extension E

	supportedVariants []string
	waterLevel        int

	ForRemotes func(
		cb func(remoteID string, remote remoteControl) error,
	) error
}

func (s *coffeeMachine[E]) BrewCoffee(
	ctx context.Context,
	variant string,
	size int,
	onProgress func(ctx context.Context, percentage int) error,
) (int, error) {
	targetID := rpc.GetRemoteID(ctx)

	defer s.ForRemotes(func(remoteID string, remote remoteControl) error {
		if remoteID == targetID {
			return nil
		}

		return remote.SetCoffeeMachineBrewing(ctx, false)
	})

	if err := s.ForRemotes(func(remoteID string, remote remoteControl) error {
		if remoteID == targetID {
			return nil
		}

		return remote.SetCoffeeMachineBrewing(ctx, true)
	}); err != nil {
		return 0, err
	}

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

	service := &coffeeMachine[*teaBrewer]{
		Extension: &teaBrewer{
			supportedVariants: []string{"darjeeling", "chai", "earlgrey"},
		},

		supportedVariants: []string{"latte", "americano"},
		waterLevel:        1000,
	}

	var clients atomic.Int64
	registry := rpc.NewRegistry[remoteControl, json.RawMessage](
		service,

		&rpc.RegistryHooks{
			OnClientConnect: func(remoteID string) {
				log.Printf("%v remote controls connected", clients.Add(1))
			},
			OnClientDisconnect: func(remoteID string) {
				log.Printf("%v remote controls connected", clients.Add(-1))
			},
		},
	)
	service.ForRemotes = registry.ForRemotes

	// Create TCP listener
	lis, err := net.Listen("tcp", "127.0.0.1:1337")
	if err != nil {
		panic(err)
	}
	defer lis.Close()

	log.Println("Listening on", lis.Addr())

	// Create HTTP server from TCP listener
	if err := http.Serve(lis, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.WriteHeader(http.StatusInternalServerError)

				log.Printf("Remote control disconnected with error: %v", err)
			}
		}()

		// Upgrade from HTTP to WebSockets
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

			// Set up the streaming JSON encoder and decoder
			encoder := json.NewEncoder(conn)
			decoder := json.NewDecoder(conn)

			go func() {
				if err := registry.LinkStream(
					r.Context(),

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
				); err != nil && !errors.Is(err, io.EOF) {
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
