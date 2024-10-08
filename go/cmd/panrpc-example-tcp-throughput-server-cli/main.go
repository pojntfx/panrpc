package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"io"
	"log"
	"net"
	"sync/atomic"

	"github.com/fxamacker/cbor/v2"
	"github.com/pojntfx/panrpc/go/pkg/rpc"
)

var (
	errUnknownSerializer = errors.New("unknown serializer")
)

type local struct {
	buf []byte
}

func (s *local) GetBytes(ctx context.Context) ([]byte, error) {
	return s.buf, nil
}

type remote struct{}

func main() {
	addr := flag.String("addr", "localhost:1337", "Listen or remote address")
	listen := flag.Bool("listen", true, "Whether to allow connecting to remotes by listening or dialing")
	serializer := flag.String("serializer", "json", "Serializer to use (json or cbor)")
	buffer := flag.Int("buffer", 1*1024*1024, "Length of the buffer to return for each RPC")

	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		clients atomic.Int64

		handleConn func(conn net.Conn) error
	)
	switch *serializer {
	case "json":
		registry := rpc.NewRegistry[remote, json.RawMessage](
			&local{
				buf: make([]byte, *buffer),
			},

			&rpc.RegistryHooks{
				OnClientConnect: func(remoteID string) {
					log.Printf("%v clients connected", clients.Add(1))
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
		registry := rpc.NewRegistry[remote, cbor.RawMessage](
			&local{
				buf: make([]byte, *buffer),
			},

			&rpc.RegistryHooks{
				OnClientConnect: func(remoteID string) {
					log.Printf("%v clients connected", clients.Add(1))
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
