package main

import (
	"bufio"
	"context"
	"flag"
	"log"
	"net"
	"os"
	"sync"
	"sync/atomic"
	"time"

	"github.com/loopholelabs/frisbee-go"
	"github.com/loopholelabs/frisbee-go/pkg/packet"
	"github.com/pojntfx/dudirekta/pkg/rpc"
	"github.com/pojntfx/r3map/pkg/utils"
)

const (
	DUDIREKTA = uint16(10)
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
		log.Println(`Enter one of the following letters followed by <ENTER> to run a function on the remote(s):

- a: Print "Hello, world!"`)

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
					if err := peer.Println(ctx, "Hello, world!"); err != nil {
						log.Println("Got error for Println func:", err)

						continue
					}
				default:
					log.Printf("Unknown letter %v, ignoring input", line)

					continue
				}
			}
		}
	}()

	handlers := make(frisbee.HandlerTable)

	var packetsLock sync.Mutex
	packets := map[string]chan []byte{}

	handlers[DUDIREKTA] = func(ctx context.Context, incoming *packet.Packet) (outgoing *packet.Packet, action frisbee.Action) {

		connID := ctx.Value("connID").(string)

		packetsLock.Lock()
		p, ok := packets[connID]
		if !ok {
			p = make(chan []byte)

			packets[connID] = p
		}
		packetsLock.Unlock()

		p <- incoming.Content.Bytes()

		return
	}

	server, err := frisbee.NewServer(handlers)
	if err != nil {
		panic(err)
	}
	defer server.Shutdown()

	server.SetOnClosed(func(a *frisbee.Async, err error) {
		connID := a.RemoteAddr().String()

		packetsLock.Lock()
		defer packetsLock.Unlock()

		p, ok := packets[connID]
		if !ok {
			return
		}

		close(p)
	})

	server.ConnContext = func(ctx context.Context, a *frisbee.Async) context.Context {
		connID := a.RemoteAddr().String()

		go func() {
			if err := registry.Link(
				func(b []byte) error {
					pkg := packet.Get()

					pkg.Metadata.Operation = DUDIREKTA
					pkg.Content.Write(b)
					pkg.Metadata.ContentLength = uint32(pkg.Content.Len())

					return a.WritePacket(pkg)
				},
				func() ([]byte, error) {
					packetsLock.Lock()
					p, ok := packets[connID]
					if !ok {
						p = make(chan []byte)

						packets[connID] = p
					}
					packetsLock.Unlock()

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

		return context.WithValue(ctx, "connID", connID)
	}

	log.Println("Listening on", *laddr)

	if err := server.Start(*laddr); err != nil && !utils.IsClosedErr(err) {
		panic(err)
	}
}
