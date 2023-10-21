package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net"
	"time"

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

	registry := rpc.NewRegistry(
		&local{},
		remote{},

		time.Second*10,
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
