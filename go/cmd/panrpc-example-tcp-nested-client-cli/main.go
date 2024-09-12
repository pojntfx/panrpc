package main

import (
	"bufio"
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"os"
	"sync/atomic"

	"github.com/pojntfx/panrpc/go/pkg/rpc"
)

type local struct{}

func (s *local) Println(ctx context.Context, msg string) error {
	log.Println("Printing message", msg, "for remote with ID", rpc.GetRemoteID(ctx))

	fmt.Println(msg)

	return nil
}

type timeRemote struct {
	GetSystemTime func(ctx context.Context) (int64, error)
}

type remote struct {
	Time timeRemote

	Increment func(ctx context.Context, delta int64) (int64, error)
}

func main() {
	addr := flag.String("addr", "localhost:1337", "Listen or remote address")
	listen := flag.Bool("listen", false, "Whether to allow connecting to remotes by listening or dialing")

	flag.Parse()

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

	go func() {
		log.Println(`Enter one of the following letters followed by <ENTER> to run a function on the remote(s):

- a: Increment remote counter by one
- b: Decrement remote counter by one
- c: Get the system time`)

		stdin := bufio.NewReader(os.Stdin)

		for {
			line, err := stdin.ReadString('\n')
			if err != nil {
				panic(err)
			}

			if err := registry.ForRemotes(func(remoteID string, remote remote) error {
				log.Println("Calling functions for remote with ID", remoteID)

				switch line {
				case "a\n":
					new, err := remote.Increment(ctx, 1)
					if err != nil {
						log.Println("Got error for Increment func:", err)

						return nil
					}

					log.Println(new)
				case "b\n":
					new, err := remote.Increment(ctx, -1)
					if err != nil {
						log.Println("Got error for Increment func:", err)

						return nil
					}

					log.Println(new)
				case "c\n":
					systemTime, err := remote.Time.GetSystemTime(ctx)
					if err != nil {
						log.Println("Got error for Time.GetSystemTime func:", err)

						return nil
					}

					log.Println(systemTime)
				default:
					log.Printf("Unknown letter %v, ignoring input", line)

					return nil
				}

				return nil
			}); err != nil {
				panic(err)
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
			conn, err := lis.Accept()
			if err != nil {
				if errors.Is(err, net.ErrClosed) {
					break
				}

				log.Println("could not accept connection, continuing:", err)

				continue
			}

			go func() {
				defer conn.Close()

				linkCtx, cancelLinkCtx := context.WithCancel(ctx)
				defer cancelLinkCtx()

				encoder := json.NewEncoder(conn)
				decoder := json.NewDecoder(conn)

				if err := registry.LinkStream(
					linkCtx,

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
					log.Println("Client disconnected with error:", err)
				}
			}()
		}
	} else {
		conn, err := net.Dial("tcp", *addr)
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		log.Println("Connected to", conn.RemoteAddr())

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

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
}
