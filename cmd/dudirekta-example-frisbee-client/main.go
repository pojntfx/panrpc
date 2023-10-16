package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"time"

	"github.com/loopholelabs/frisbee-go"
	"github.com/loopholelabs/frisbee-go/pkg/packet"
	"github.com/pojntfx/dudirekta/pkg/rpc"
	"github.com/pojntfx/r3map/pkg/utils"
)

const (
	DUDIREKTA_REQUESTS  = uint16(10)
	DUDIREKTA_RESPONSES = uint16(11)
)

type local struct{}

func (s *local) Println(ctx context.Context, msg string) error {
	log.Println("Printing message", msg, "for peer with ID", rpc.GetRemoteID(ctx))

	fmt.Println(msg)

	return nil
}

type remote struct {
	Increment func(ctx context.Context, delta int64) (int64, error)
}

func main() {
	raddr := flag.String("raddr", "localhost:1337", "Remote address")

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
				new, err := peer.Increment(ctx, 1)
				if err != nil {
					log.Println("Got error for Increment func:", err)

					continue
				}

				log.Println(new)

				new, err = peer.Increment(ctx, -1)
				if err != nil {
					log.Println("Got error for Increment func:", err)

					continue
				}

				log.Println(new)
			}
		}
	}()

	handlers := make(frisbee.HandlerTable)

	requestPackets := make(chan []byte)
	handlers[DUDIREKTA_REQUESTS] = func(ctx context.Context, incoming *packet.Packet) (outgoing *packet.Packet, action frisbee.Action) {
		b := make([]byte, incoming.Metadata.ContentLength)
		copy(b, incoming.Content.Bytes())
		requestPackets <- b

		return
	}

	responsePackets := make(chan []byte)
	handlers[DUDIREKTA_RESPONSES] = func(ctx context.Context, incoming *packet.Packet) (outgoing *packet.Packet, action frisbee.Action) {
		b := make([]byte, incoming.Metadata.ContentLength)
		copy(b, incoming.Content.Bytes())
		responsePackets <- b

		return
	}

	client, err := frisbee.NewClient(handlers, ctx)
	if err != nil {
		panic(err)
	}
	defer client.Close()

	if err := client.Connect(*raddr); err != nil {
		panic(err)
	}

	log.Println("Connected to", *raddr)

	go func() {
		if err := registry.LinkMessage(
			func(b []byte) error {
				pkg := packet.Get()

				pkg.Metadata.Operation = DUDIREKTA_REQUESTS
				pkg.Content.Write(b)
				pkg.Metadata.ContentLength = uint32(pkg.Content.Len())

				return client.WritePacket(pkg)
			},
			func(b []byte) error {
				pkg := packet.Get()

				pkg.Metadata.Operation = DUDIREKTA_RESPONSES
				pkg.Content.Write(b)
				pkg.Metadata.ContentLength = uint32(pkg.Content.Len())

				return client.WritePacket(pkg)
			},

			func() ([]byte, error) {
				b, ok := <-requestPackets
				if !ok {
					return []byte{}, net.ErrClosed
				}

				return b, nil
			},
			func() ([]byte, error) {
				b, ok := <-responsePackets
				if !ok {
					return []byte{}, net.ErrClosed
				}

				return b, nil
			},

			json.Marshal,
			json.Unmarshal,
		); err != nil && !utils.IsClosedErr(err) {
			panic(err)
		}
	}()

	<-client.CloseChannel()

	close(requestPackets)
	close(responsePackets)
}
