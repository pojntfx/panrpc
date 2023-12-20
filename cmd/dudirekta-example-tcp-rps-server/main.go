package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net"

	"github.com/fxamacker/cbor/v2"
	"github.com/pojntfx/dudirekta/pkg/rpc"
)

var (
	errUnknownSerializer = errors.New("unknown serializer")
)

type local struct{}

func (s *local) ZeroInt(ctx context.Context) (int, error) {
	return 0, nil
}

func (s *local) ZeroInt8(ctx context.Context) (int8, error) {
	return 0, nil
}

func (s *local) ZeroInt16(ctx context.Context) (int16, error) {
	return 0, nil
}

func (s *local) ZeroInt32(ctx context.Context) (int32, error) {
	return 0, nil
}

func (s *local) ZeroRune(ctx context.Context) (rune, error) {
	return 0, nil
}

func (s *local) ZeroInt64(ctx context.Context) (int64, error) {
	return 0, nil
}

func (s *local) ZeroUint(ctx context.Context) (uint, error) {
	return 0, nil
}

func (s *local) ZeroUint8(ctx context.Context) (uint8, error) {
	return 0, nil
}

func (s *local) ZeroByte(ctx context.Context) (byte, error) {
	return 0, nil
}

func (s *local) ZeroUint16(ctx context.Context) (uint16, error) {
	return 0, nil
}

func (s *local) ZeroUint32(ctx context.Context) (uint32, error) {
	return 0, nil
}

func (s *local) ZeroUint64(ctx context.Context) (uint64, error) {
	return 0, nil
}

func (s *local) ZeroUintptr(ctx context.Context) (uintptr, error) {
	return 0, nil
}

func (s *local) ZeroFloat32(ctx context.Context) (float32, error) {
	return 0, nil
}

func (s *local) ZeroFloat64(ctx context.Context) (float64, error) {
	return 0, nil
}

func (s *local) ZeroComplex64(ctx context.Context) (complex64, error) {
	return 0, nil
}

func (s *local) ZeroComplex128(ctx context.Context) (complex128, error) {
	return 0, nil
}

func (s *local) ZeroBool(ctx context.Context) (bool, error) {
	return false, nil
}

func (s *local) ZeroString(ctx context.Context) (string, error) {
	return "", nil
}

func (s *local) ZeroArray(ctx context.Context) ([]interface{}, error) {
	return nil, nil
}

func (s *local) ZeroSlice(ctx context.Context) ([]interface{}, error) {
	return nil, nil
}

func (s *local) ZeroStruct(ctx context.Context) (interface{}, error) {
	return struct{}{}, nil
}

type remote struct{}

func main() {
	addr := flag.String("addr", "localhost:1337", "Listen or remote address")
	listen := flag.Bool("listen", true, "Whether to allow connecting to remotes by listening or dialing")
	serializer := flag.String("serializer", "json", "Serializer to use (json or cbor)")

	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		clients = 0

		handleConn func(conn net.Conn) error
	)
	switch *serializer {
	case "json":
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
		registry := rpc.NewRegistry[remote, cbor.RawMessage](
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
