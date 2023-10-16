package main

import (
	"context"
	"flag"
	"log"
	"net"
	"sync"
	"sync/atomic"
	"time"

	"github.com/loopholelabs/frisbee-go"
	"github.com/loopholelabs/frisbee-go/pkg/packet"
	"github.com/pojntfx/dudirekta/pkg/rpc"
	"github.com/pojntfx/r3map/pkg/utils"
)

type Key int

const (
	DUDIREKTA_REQUESTS  = uint16(10)
	DUDIREKTA_RESPONSES = uint16(11)

	ConnIDKey Key = iota
)

type local struct {
	counter int64
}

func (s *local) Increment(ctx context.Context, delta int64) (int64, error) {
	log.Println("Incrementing counter by", delta, "for peer with ID", rpc.GetRemoteID(ctx))

	return atomic.AddInt64(&s.counter, delta), nil
}

type remote struct {
	Println func(ctx context.Context, msg string) error
}

func main() {
	laddr := flag.String("laddr", "localhost:1337", "Listen address")

	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clients := 0

	registry := rpc.NewRegistry(
		&local{},
		remote{},

		time.Second*10,
		ctx,
		&rpc.Options{
			ResponseBufferLen: rpc.DefaultResponseBufferLen,
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
		for {
			for _, peer := range registry.Peers() {
				if err := peer.Println(ctx, "Hello, world!"); err != nil {
					log.Println("Got error for Println func:", err)

					continue
				}
			}
		}
	}()

	handlers := make(frisbee.HandlerTable)

	var requestPacketsLock sync.Mutex
	requestPackets := map[string]chan []byte{}

	handlers[DUDIREKTA_REQUESTS] = func(ctx context.Context, incoming *packet.Packet) (outgoing *packet.Packet, action frisbee.Action) {
		connID := ctx.Value(ConnIDKey).(string)

		requestPacketsLock.Lock()
		p, ok := requestPackets[connID]
		if !ok {
			p = make(chan []byte)

			requestPackets[connID] = p
		}
		requestPacketsLock.Unlock()

		b := make([]byte, incoming.Metadata.ContentLength)
		copy(b, incoming.Content.Bytes())
		p <- b

		return
	}

	var responsePacketsLock sync.Mutex
	responsePackets := map[string]chan []byte{}

	handlers[DUDIREKTA_RESPONSES] = func(ctx context.Context, incoming *packet.Packet) (outgoing *packet.Packet, action frisbee.Action) {
		connID := ctx.Value(ConnIDKey).(string)

		responsePacketsLock.Lock()
		p, ok := responsePackets[connID]
		if !ok {
			p = make(chan []byte)

			responsePackets[connID] = p
		}
		responsePacketsLock.Unlock()

		b := make([]byte, incoming.Metadata.ContentLength)
		copy(b, incoming.Content.Bytes())
		p <- b

		return
	}

	server, err := frisbee.NewServer(handlers)
	if err != nil {
		panic(err)
	}
	defer server.Shutdown()

	server.SetOnClosed(func(a *frisbee.Async, err error) {
		connID := a.RemoteAddr().String()

		requestPacketsLock.Lock()
		defer requestPacketsLock.Unlock()

		rqp, ok := requestPackets[connID]
		if !ok {
			return
		}

		close(rqp)

		responsePacketsLock.Lock()
		defer responsePacketsLock.Unlock()

		rsp, ok := responsePackets[connID]
		if !ok {
			return
		}

		close(rsp)
	})

	server.ConnContext = func(ctx context.Context, a *frisbee.Async) context.Context {
		connID := a.RemoteAddr().String()

		go func() {
			if err := registry.Link(
				func(b []byte) error {
					pkg := packet.Get()

					pkg.Metadata.Operation = DUDIREKTA_REQUESTS
					pkg.Content.Write(b)
					pkg.Metadata.ContentLength = uint32(pkg.Content.Len())

					return a.WritePacket(pkg)
				},
				func(b []byte) error {
					pkg := packet.Get()

					pkg.Metadata.Operation = DUDIREKTA_RESPONSES
					pkg.Content.Write(b)
					pkg.Metadata.ContentLength = uint32(pkg.Content.Len())

					return a.WritePacket(pkg)
				},

				func() ([]byte, error) {
					requestPacketsLock.Lock()
					p, ok := requestPackets[connID]
					if !ok {
						p = make(chan []byte)

						requestPackets[connID] = p
					}
					requestPacketsLock.Unlock()

					b, ok := <-p
					if !ok {
						return []byte{}, net.ErrClosed
					}

					return b, nil
				},
				func() ([]byte, error) {
					responsePacketsLock.Lock()
					p, ok := responsePackets[connID]
					if !ok {
						p = make(chan []byte)

						responsePackets[connID] = p
					}
					responsePacketsLock.Unlock()

					b, ok := <-p
					if !ok {
						return []byte{}, net.ErrClosed
					}

					return b, nil
				},
			); err != nil && !utils.IsClosedErr(err) {
				panic(err)
			}
		}()

		return context.WithValue(ctx, ConnIDKey, connID)
	}

	log.Println("Listening on", *laddr)

	if err := server.Start(*laddr); err != nil && !utils.IsClosedErr(err) {
		panic(err)
	}
}
