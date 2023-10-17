package utils

import (
	"context"
	"net"
	"sync"

	"github.com/loopholelabs/frisbee-go"
	"github.com/loopholelabs/frisbee-go/pkg/packet"
)

type FrisbeeDialer struct {
	raddr string

	requestOperation  uint16
	responseOperation uint16

	client *frisbee.Client

	requestPackets  chan []byte
	responsePackets chan []byte

	wg sync.WaitGroup
}

func NewFrisbeeDialer(
	raddr string,

	requestOperation uint16,
	responseOperation uint16,
) *FrisbeeDialer {
	return &FrisbeeDialer{
		raddr: raddr,

		requestOperation:  requestOperation,
		responseOperation: responseOperation,

		requestPackets:  make(chan []byte),
		responsePackets: make(chan []byte),

		wg: sync.WaitGroup{},
	}
}

func (d *FrisbeeDialer) Connect(ctx context.Context) error {
	handlers := make(frisbee.HandlerTable)

	handlers[d.requestOperation] = func(ctx context.Context, incoming *packet.Packet) (outgoing *packet.Packet, action frisbee.Action) {
		b := make([]byte, incoming.Metadata.ContentLength)
		copy(b, incoming.Content.Bytes())
		d.requestPackets <- b

		return
	}

	handlers[d.responseOperation] = func(ctx context.Context, incoming *packet.Packet) (outgoing *packet.Packet, action frisbee.Action) {
		b := make([]byte, incoming.Metadata.ContentLength)
		copy(b, incoming.Content.Bytes())
		d.responsePackets <- b

		return
	}

	var err error
	d.client, err = frisbee.NewClient(handlers, ctx)
	if err != nil {
		return err
	}

	return d.client.Connect(d.raddr)
}

func (d *FrisbeeDialer) Link(
	ctx context.Context,

	linkMessage func(
		writeRequest,
		writeResponse func(b []byte) error,

		readRequest,
		readResponse func() ([]byte, error),

		marshal func(v any) ([]byte, error),
		unmarshal func(data []byte, v any) error,
	) error,

	marshal func(v any) ([]byte, error),
	unmarshal func(data []byte, v any) error,

	onLinkError func(ctx context.Context, c *frisbee.Client, err error),
) error {
	d.wg.Add(1)
	go func() {
		defer d.wg.Done()

		if err := linkMessage(
			func(b []byte) error {
				pkg := packet.Get()

				pkg.Metadata.Operation = d.requestOperation
				pkg.Content.Write(b)
				pkg.Metadata.ContentLength = uint32(pkg.Content.Len())

				return d.client.WritePacket(pkg)
			},
			func(b []byte) error {
				pkg := packet.Get()

				pkg.Metadata.Operation = d.responseOperation
				pkg.Content.Write(b)
				pkg.Metadata.ContentLength = uint32(pkg.Content.Len())

				return d.client.WritePacket(pkg)
			},

			func() ([]byte, error) {
				b, ok := <-d.requestPackets
				if !ok {
					return []byte{}, net.ErrClosed
				}

				return b, nil
			},
			func() ([]byte, error) {
				b, ok := <-d.responsePackets
				if !ok {
					return []byte{}, net.ErrClosed
				}

				return b, nil
			},

			marshal,
			unmarshal,
		); err != nil {
			onLinkError(ctx, d.client, err)
		}
	}()

	<-d.client.CloseChannel()

	return nil
}

func (d *FrisbeeDialer) Close() error {
	if d.client != nil {
		_ = d.client.Close()

		close(d.requestPackets)
		close(d.responsePackets)

		d.wg.Wait()

		d.client = nil
	}

	return nil
}
