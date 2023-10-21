package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/pojntfx/dudirekta/pkg/rpc"
)

var (
	errUnknownSerializer = errors.New("unknown serializer")
)

type local struct{}

type remote struct {
	Example func(ctx context.Context, in int64) (int64, error)
}

func main() {
	addr := flag.String("addr", "localhost:1337", "Listen or remote address")
	listen := flag.Bool("listen", false, "Whether to allow connecting to remotes by listening or dialing")
	concurrency := flag.Int("concurrency", 512, "Amount of concurrent calls to allow per client")
	serializer := flag.String("serializer", "json", "Serializer to use (json or cbor)")

	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		getEncoder func(conn net.Conn) func(v any) error
		getDecoder func(conn net.Conn) func(v any) error

		marshal   func(v any) ([]byte, error)
		unmarshal func(data []byte, v any) error
	)
	switch *serializer {
	case "json":
		getEncoder = func(conn net.Conn) func(v any) error {
			return json.NewEncoder(conn).Encode
		}
		getDecoder = func(conn net.Conn) func(v any) error {
			return json.NewDecoder(conn).Decode
		}

		marshal = json.Marshal
		unmarshal = json.Unmarshal

	case "cbor":
		getEncoder = func(conn net.Conn) func(v any) error {
			return cbor.NewEncoder(conn).Encode
		}
		getDecoder = func(conn net.Conn) func(v any) error {
			return cbor.NewDecoder(conn).Decode
		}

		marshal = json.Marshal
		unmarshal = json.Unmarshal

	default:
		panic(errUnknownSerializer)
	}

	clients := 0

	var registry *rpc.Registry[remote]
	registry = rpc.NewRegistry(
		&local{},
		remote{},

		time.Second*10,
		ctx,
		&rpc.Options{
			OnClientConnect: func(remoteID string) {
				clients++

				log.Printf("%v clients connected", clients)

				go func() {
					rps := new(atomic.Int64)
					go func() {
						ticker := time.NewTicker(time.Second)
						defer ticker.Stop()

						for range ticker.C {
							log.Println(rps.Load(), "requests/s")

							rps.Store(0)

							time.Sleep(time.Second)
						}
					}()

					var wg sync.WaitGroup
					if err := registry.ForRemotes(func(remoteID string, r remote) error {
						for i := 0; i < *concurrency; i++ {
							wg.Add(1)

							go func(r remote) {
								defer wg.Done()

								for {
									if _, err := r.Example(ctx, 1); err != nil {
										panic(err)
									}

									rps.Add(1)
								}
							}(r)
						}

						return nil
					}); err != nil {
						panic(err)
					}

					wg.Wait()
				}()
			},
			OnClientDisconnect: func(remoteID string) {
				clients--

				log.Printf("%v clients connected", clients)
			},
		},
	)

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

					if err := registry.LinkStream(
						getEncoder(conn),
						getDecoder(conn),

						marshal,
						unmarshal,
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

		if err := registry.LinkStream(
			getEncoder(conn),
			getDecoder(conn),

			marshal,
			unmarshal,
		); err != nil {
			panic(err)
		}
	}
}
