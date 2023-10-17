package main

import (
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"time"

	"github.com/loopholelabs/frisbee-go"
	"github.com/pojntfx/dudirekta/pkg/rpc"
	iutils "github.com/pojntfx/dudirekta/pkg/utils"
	"github.com/pojntfx/r3map/pkg/utils"
)

const (
	DUDIREKTA_REQUEST  = uint16(10)
	DUDIREKTA_RESPONSE = uint16(11)
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

	d := iutils.NewFrisbeeDialer(*raddr, DUDIREKTA_REQUEST, DUDIREKTA_RESPONSE)

	if err := d.Connect(ctx); err != nil {
		panic(err)
	}
	defer d.Close()

	log.Println("Connected to", *raddr)

	if err := d.Link(
		ctx,

		registry.LinkMessage,

		json.Marshal,
		json.Unmarshal,

		func(ctx context.Context, c *frisbee.Client, err error) {
			if err != nil && !utils.IsClosedErr(err) {
				log.Println("Could not link registry:", err)
			}
		},
	); err != nil && !utils.IsClosedErr(err) {
		panic(err)
	}
}
