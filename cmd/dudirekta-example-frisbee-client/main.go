package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"
	"time"

	"github.com/loopholelabs/frisbee-go"
	"github.com/loopholelabs/frisbee-go/pkg/packet"
	"github.com/pojntfx/dudirekta/pkg/rpc"
	"github.com/pojntfx/r3map/pkg/utils"
)

const (
	DUDIREKTA = uint16(10)
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
		log.Println(`Enter one of the following letters followed by <ENTER> to run a function on the remote(s):

- a: Increment remote counter by one
- b: Decrement remote counter by one`)

		stdin := bufio.NewReader(os.Stdin)

		for {
			line, err := stdin.ReadString('\n')
			if err != nil {
				panic(err)
			}

			for peerID, peer := range registry.Peers() {
				log.Println("Calling functions for peer with ID", peerID)

				switch line {
				case "a\n":
					new, err := peer.Increment(ctx, 1)
					if err != nil {
						log.Println("Got error for Increment func:", err)

						continue
					}

					log.Println(new)
				case "b\n":
					new, err := peer.Increment(ctx, -1)
					if err != nil {
						log.Println("Got error for Increment func:", err)

						continue
					}

					log.Println(new)
				default:
					log.Printf("Unknown letter %v, ignoring input", line)

					continue
				}
			}
		}
	}()

	handlers := make(frisbee.HandlerTable)

	packets := make(chan []byte)
	handlers[DUDIREKTA] = func(ctx context.Context, incoming *packet.Packet) (outgoing *packet.Packet, action frisbee.Action) {
		packets <- incoming.Content.Bytes()

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
		if err := registry.Link(
			func(b []byte) error {
				pkg := packet.Get()

				pkg.Metadata.Operation = DUDIREKTA
				pkg.Content.Write(b)
				pkg.Metadata.ContentLength = uint32(pkg.Content.Len())

				return client.WritePacket(pkg)
			},
			func() ([]byte, error) {
				b, ok := <-packets
				if !ok {
					return []byte{}, net.ErrClosed
				}

				return b, nil
			},
		); err != nil && !utils.IsClosedErr(err) {
			panic(err)
		}
	}()

	<-client.CloseChannel()

	close(packets)
}
