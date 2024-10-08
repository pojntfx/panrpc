package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"math"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/pojntfx/panrpc/go/pkg/rpc"
)

var (
	errUnknownSerializer = errors.New("unknown serializer")
)

type local struct{}

type remote struct {
	GetBytes func(ctx context.Context) ([]byte, error)
}

func main() {
	addr := flag.String("addr", "localhost:1337", "Listen or remote address")
	listen := flag.Bool("listen", false, "Whether to allow connecting to remotes by listening or dialing")
	concurrency := flag.Int("concurrency", 512, "Amount of concurrent calls to allow per client")
	serializer := flag.String("serializer", "json", "Serializer to use (json or cbor)")
	runs := flag.Int("runs", 10, "Amount of test runs to do before exiting")

	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var clients atomic.Int64
	onClientConnect := func(forRemotes func(func(remoteID string, r remote) error) error, remoteID string) {
		log.Printf("%v clients connected", clients.Add(1))

		go func() {
			bytesTransferred := new(atomic.Int64)
			currentRuns := 0
			go func() {
				ticker := time.NewTicker(time.Second)
				defer ticker.Stop()

				for range ticker.C {
					fmt.Println(math.RoundToEven(float64(bytesTransferred.Load())/float64(1024*1024)), "MB/s")

					bytesTransferred.Store(0)

					currentRuns++
					if currentRuns >= *runs {
						os.Exit(0)

						return
					}

					time.Sleep(time.Second)
				}
			}()

			var wg sync.WaitGroup
			if err := forRemotes(func(remoteID string, r remote) error {
				for i := 0; i < *concurrency; i++ {
					wg.Add(1)

					go func(r remote) {
						defer wg.Done()

						for {
							buf, err := r.GetBytes(ctx)
							if err != nil {
								panic(err)
							}

							bytesTransferred.Add(int64(len(buf)))
						}
					}(r)
				}

				return nil
			}); err != nil {
				panic(err)
			}

			wg.Wait()
		}()
	}

	var handleConn func(conn net.Conn) error
	switch *serializer {
	case "json":
		var registry *rpc.Registry[remote, json.RawMessage]
		registry = rpc.NewRegistry[remote, json.RawMessage](
			&local{},

			&rpc.RegistryHooks{
				OnClientConnect: func(remoteID string) {
					onClientConnect(registry.ForRemotes, remoteID)
				},
				OnClientDisconnect: func(remoteID string) {
					log.Printf("%v clients connected", clients.Add(-1))
				},
			},
		)

		handleConn = func(conn net.Conn) error {
			linkCtx, cancelLinkCtx := context.WithCancel(ctx)
			defer cancelLinkCtx()

			encoder := json.NewEncoder(conn)
			decoder := json.NewDecoder(conn)

			return registry.LinkStream(
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
			)
		}

	case "cbor":
		var registry *rpc.Registry[remote, cbor.RawMessage]
		registry = rpc.NewRegistry[remote, cbor.RawMessage](
			&local{},

			&rpc.RegistryHooks{
				OnClientConnect: func(remoteID string) {
					onClientConnect(registry.ForRemotes, remoteID)
				},
				OnClientDisconnect: func(remoteID string) {
					log.Printf("%v clients connected", clients.Add(-1))
				},
			},
		)

		handleConn = func(conn net.Conn) error {
			linkCtx, cancelLinkCtx := context.WithCancel(ctx)
			defer cancelLinkCtx()

			encoder := cbor.NewEncoder(conn)
			decoder := cbor.NewDecoder(conn)

			return registry.LinkStream(
				linkCtx,

				func(v rpc.Message[cbor.RawMessage]) error {
					return encoder.Encode(v)
				},
				func(v *rpc.Message[cbor.RawMessage]) error {
					return decoder.Decode(v)
				},

				func(v any) (cbor.RawMessage, error) {
					b, err := cbor.Marshal(v)
					if err != nil {
						return nil, err
					}

					return cbor.RawMessage(b), nil
				},
				func(data cbor.RawMessage, v any) error {
					return cbor.Unmarshal([]byte(data), v)
				},

				nil,
			)
		}

	default:
		panic(errUnknownSerializer)
	}

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

				if err := handleConn(conn); err != nil && !errors.Is(err, io.EOF) {
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

		if err := handleConn(conn); err != nil {
			panic(err)
		}
	}
}
