package main

import (
	"context"
	"encoding/json"
	"flag"
	"log"
	"sync/atomic"
	"time"

	"github.com/loopholelabs/frisbee-go"
	"github.com/pojntfx/dudirekta/pkg/rpc"
	iutils "github.com/pojntfx/dudirekta/pkg/utils"
	"github.com/pojntfx/r3map/pkg/utils"
)

type Key int

const (
	DUDIREKTA_REQUEST  = uint16(10)
	DUDIREKTA_RESPONSE = uint16(11)

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

	l := iutils.NewFrisbeeListener(*laddr, DUDIREKTA_REQUEST, DUDIREKTA_RESPONSE)

	if err := l.Listen(); err != nil {
		panic(err)
	}
	defer l.Close()

	log.Println("Listening on", *laddr)

	if err := l.Link(
		ctx,

		registry.LinkMessage,

		json.Marshal,
		json.Unmarshal,

		func(ctx context.Context, a *frisbee.Async, err error) {
			if err != nil && !utils.IsClosedErr(err) {
				log.Println("Could not link registry:", err)
			}
		},
	); err != nil && !utils.IsClosedErr(err) {
		panic(err)
	}
}
