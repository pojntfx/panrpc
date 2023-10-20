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
	jsoniter "github.com/json-iterator/go"
	"github.com/pojntfx/dudirekta/pkg/rpc"
	"github.com/vmihailenco/msgpack/v5"
)

var (
	errUnknownSerializer = errors.New("unknown serializer")
)

type local struct{}

func (s *local) Example(ctx context.Context, in int64) (int64, error) {
	return 0, nil
}

type remote struct{}

func main() {
	addr := flag.String("addr", "localhost:1337", "Listen or remote address")
	listen := flag.Bool("listen", true, "Whether to allow connecting to remotes by listening or dialing")
	serializer := flag.String("serializer", "json", "Serializer to use (one of json, json-iterator, cbor or msgpack)")

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

	case "json-iterator":
		getEncoder = func(conn net.Conn) func(v any) error {
			return jsoniter.ConfigCompatibleWithStandardLibrary.NewEncoder(conn).Encode
		}
		getDecoder = func(conn net.Conn) func(v any) error {
			return jsoniter.ConfigCompatibleWithStandardLibrary.NewDecoder(conn).Decode
		}

		marshal = jsoniter.ConfigCompatibleWithStandardLibrary.Marshal
		unmarshal = jsoniter.ConfigCompatibleWithStandardLibrary.Unmarshal

	case "cbor":
		getEncoder = func(conn net.Conn) func(v any) error {
			return cbor.NewEncoder(conn).Encode
		}
		getDecoder = func(conn net.Conn) func(v any) error {
			return cbor.NewDecoder(conn).Decode
		}

		marshal = json.Marshal
		unmarshal = json.Unmarshal

	case "msgpack":
		getEncoder = func(conn net.Conn) func(v any) error {
			return msgpack.NewEncoder(conn).Encode
		}
		getDecoder = func(conn net.Conn) func(v any) error {
			return msgpack.NewDecoder(conn).Decode
		}

		marshal = msgpack.Marshal
		unmarshal = msgpack.Unmarshal

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
