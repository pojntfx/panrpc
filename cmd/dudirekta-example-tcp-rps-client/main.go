package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/fxamacker/cbor/v2"
	"github.com/pojntfx/dudirekta/pkg/rpc"
)

var (
	errUnknownSerializer = errors.New("unknown serializer")
	errUnknownDataType   = errors.New("unknown data type")
)

type local struct{}

type remote struct {
	ZeroInt        func(ctx context.Context) (int, error)
	ZeroInt8       func(ctx context.Context) (int8, error)
	ZeroInt16      func(ctx context.Context) (int16, error)
	ZeroInt32      func(ctx context.Context) (int32, error)
	ZeroRune       func(ctx context.Context) (rune, error)
	ZeroInt64      func(ctx context.Context) (int64, error)
	ZeroUint       func(ctx context.Context) (uint, error)
	ZeroUint8      func(ctx context.Context) (uint8, error)
	ZeroByte       func(ctx context.Context) (byte, error)
	ZeroUint16     func(ctx context.Context) (uint16, error)
	ZeroUint32     func(ctx context.Context) (uint32, error)
	ZeroUint64     func(ctx context.Context) (uint64, error)
	ZeroUintptr    func(ctx context.Context) (uintptr, error)
	ZeroFloat32    func(ctx context.Context) (float32, error)
	ZeroFloat64    func(ctx context.Context) (float64, error)
	ZeroComplex64  func(ctx context.Context) (complex64, error)
	ZeroComplex128 func(ctx context.Context) (complex128, error)
	ZeroBool       func(ctx context.Context) (bool, error)
	ZeroString     func(ctx context.Context) (string, error)
	ZeroArray      func(ctx context.Context) ([]interface{}, error)
	ZeroSlice      func(ctx context.Context) ([]interface{}, error)
	ZeroStruct     func(ctx context.Context) (interface{}, error)
}

func main() {
	addr := flag.String("addr", "localhost:1337", "Listen or remote address")
	listen := flag.Bool("listen", false, "Whether to allow connecting to remotes by listening or dialing")
	concurrency := flag.Int("concurrency", 512, "Amount of concurrent calls to allow per client")
	serializer := flag.String("serializer", "json", "Serializer to use (json or cbor)")
	dataType := flag.String("data-type", "int", "Data type to test one of int, int8, int16, int32, rune, int64, uint, uint8, byte, uint16, uint32, uint64, uintptr, float32, float64, complex64, complex128, bool, string, array, slice, struct")
	runs := flag.Int("runs", 10, "Amount of test runs to do before exiting")

	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clients := 0
	onClientConnect := func(forRemotes func(func(remoteID string, r remote) error) error, remoteID string) {
		clients++

		log.Printf("%v clients connected", clients)

		go func() {
			rps := new(atomic.Int64)
			currentRuns := 0
			go func() {
				ticker := time.NewTicker(time.Second)
				defer ticker.Stop()

				for range ticker.C {
					fmt.Println(rps.Load(), "requests/s")

					rps.Store(0)

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
							switch *dataType {
							case "int":
								if _, err := r.ZeroInt(ctx); err != nil {
									panic(err)
								}

							case "int8":
								if _, err := r.ZeroInt8(ctx); err != nil {
									panic(err)
								}

							case "int16":
								if _, err := r.ZeroInt16(ctx); err != nil {
									panic(err)
								}

							case "int32":
								if _, err := r.ZeroInt32(ctx); err != nil {
									panic(err)
								}

							case "rune":
								if _, err := r.ZeroRune(ctx); err != nil {
									panic(err)
								}

							case "int64":
								if _, err := r.ZeroInt64(ctx); err != nil {
									panic(err)
								}

							case "uint":
								if _, err := r.ZeroUint(ctx); err != nil {
									panic(err)
								}

							case "uint8":
								if _, err := r.ZeroUint8(ctx); err != nil {
									panic(err)
								}

							case "byte":
								if _, err := r.ZeroByte(ctx); err != nil {
									panic(err)
								}

							case "uint16":
								if _, err := r.ZeroUint16(ctx); err != nil {
									panic(err)
								}

							case "uint32":
								if _, err := r.ZeroUint32(ctx); err != nil {
									panic(err)
								}

							case "uint64":
								if _, err := r.ZeroUint64(ctx); err != nil {
									panic(err)
								}

							case "uintptr":
								if _, err := r.ZeroUintptr(ctx); err != nil {
									panic(err)
								}

							case "float32":
								if _, err := r.ZeroFloat32(ctx); err != nil {
									panic(err)
								}

							case "float64":
								if _, err := r.ZeroFloat64(ctx); err != nil {
									panic(err)
								}

							case "complex64":
								if _, err := r.ZeroComplex64(ctx); err != nil {
									panic(err)
								}

							case "complex128":
								if _, err := r.ZeroComplex128(ctx); err != nil {
									panic(err)
								}

							case "bool":
								if _, err := r.ZeroBool(ctx); err != nil {
									panic(err)
								}

							case "string":
								if _, err := r.ZeroString(ctx); err != nil {
									panic(err)
								}

							case "array":
								if _, err := r.ZeroArray(ctx); err != nil {
									panic(err)
								}

							case "slice":
								if _, err := r.ZeroSlice(ctx); err != nil {
									panic(err)
								}

							case "struct":
								if _, err := r.ZeroStruct(ctx); err != nil {
									panic(err)
								}

							default:
								panic(errUnknownDataType)
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
	}

	var handleConn func(conn net.Conn) error
	switch *serializer {
	case "json":
		var registry *rpc.Registry[remote, json.RawMessage]
		registry = rpc.NewRegistry[remote, json.RawMessage](
			&local{},

			time.Second*10,
			ctx,
			&rpc.Options{
				OnClientConnect: func(remoteID string) {
					onClientConnect(registry.ForRemotes, remoteID)
				},
				OnClientDisconnect: func(remoteID string) {
					clients--

					log.Printf("%v clients connected", clients)
				},
			},
		)

		handleConn = func(conn net.Conn) error {
			encoder := json.NewEncoder(conn)
			decoder := json.NewDecoder(conn)

			return registry.LinkStream(
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
			)
		}

	case "cbor":
		var registry *rpc.Registry[remote, cbor.RawMessage]
		registry = rpc.NewRegistry[remote, cbor.RawMessage](
			&local{},

			time.Second*10,
			ctx,
			&rpc.Options{
				OnClientConnect: func(remoteID string) {
					onClientConnect(registry.ForRemotes, remoteID)
				},
				OnClientDisconnect: func(remoteID string) {
					clients--

					log.Printf("%v clients connected", clients)
				},
			},
		)

		handleConn = func(conn net.Conn) error {
			encoder := cbor.NewEncoder(conn)
			decoder := cbor.NewDecoder(conn)

			return registry.LinkStream(
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

					if err := handleConn(conn); err != nil {
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

		if err := handleConn(conn); err != nil {
			panic(err)
		}
	}
}
