package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/pojntfx/dudirekta/pkg/rpc"
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
	addr := flag.String("addr", "localhost:1337", "Listen or remote address")
	listen := flag.Bool("listen", false, "Whether to allow connecting to remotes by listening or dialing")

	flag.Parse()

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

	go func() {
		log.Println(`Enter one of the following letters followed by <ENTER> to run a function on the remote(s):

- a: Increment remote counter by one
- b: Decrement remote counter by one`)

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

					encoder := json.NewEncoder(conn)
					decoder := json.NewDecoder(conn)

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

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

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
}
