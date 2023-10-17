package utils

import (
	"context"
	"net"
	"sync"

	"github.com/loopholelabs/frisbee-go"
	"github.com/loopholelabs/frisbee-go/pkg/packet"
)

type Key int

const (
	ConnIDKey Key = iota
)

type FrisbeeListener struct {
	laddr string

	requestOperation  uint16
	responseOperation uint16

	lis    net.Listener
	server *frisbee.Server

	wg sync.WaitGroup
}

func NewFrisbeeListener(
	laddr string,

	requestOperation uint16,
	responseOperation uint16,
) *FrisbeeListener {
	return &FrisbeeListener{
		laddr: laddr,

		requestOperation:  requestOperation,
		responseOperation: responseOperation,

		wg: sync.WaitGroup{},
	}
}

func (l *FrisbeeListener) Listen() error {
	var err error
	l.lis, err = net.Listen("tcp", l.laddr)
	if err != nil {
		return err
	}

	return nil
}

func (l *FrisbeeListener) Link(
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

	onLinkError func(ctx context.Context, a *frisbee.Async, err error),
) error {
	handlers := make(frisbee.HandlerTable)

	var requestPacketsLock sync.Mutex
	requestPackets := map[string]chan []byte{}

	handlers[l.requestOperation] = func(ctx context.Context, incoming *packet.Packet) (outgoing *packet.Packet, action frisbee.Action) {
		connID := ctx.Value(ConnIDKey).(string)

		requestPacketsLock.Lock()
		defer requestPacketsLock.Unlock()

		p, ok := requestPackets[connID]
		if !ok {
			p = make(chan []byte)

			requestPackets[connID] = p
		}

		b := make([]byte, incoming.Metadata.ContentLength)
		copy(b, incoming.Content.Bytes())
		p <- b

		return
	}

	var responsePacketsLock sync.Mutex
	responsePackets := map[string]chan []byte{}

	handlers[l.responseOperation] = func(ctx context.Context, incoming *packet.Packet) (outgoing *packet.Packet, action frisbee.Action) {
		connID := ctx.Value(ConnIDKey).(string)

		responsePacketsLock.Lock()
		defer responsePacketsLock.Unlock()

		p, ok := responsePackets[connID]
		if !ok {
			p = make(chan []byte)

			responsePackets[connID] = p
		}

		b := make([]byte, incoming.Metadata.ContentLength)
		copy(b, incoming.Content.Bytes())
		p <- b

		return
	}

	var err error
	l.server, err = frisbee.NewServer(handlers)
	if err != nil {
		return err
	}

	l.server.SetOnClosed(func(a *frisbee.Async, err error) {
		connID := a.RemoteAddr().String()

		requestPacketsLock.Lock()
		defer requestPacketsLock.Unlock()

		rqp, ok := requestPackets[connID]
		if !ok {
			return
		}

		close(rqp)
		delete(requestPackets, connID)

		responsePacketsLock.Lock()
		defer responsePacketsLock.Unlock()

		rsp, ok := responsePackets[connID]
		if !ok {
			return
		}

		close(rsp)
		delete(responsePackets, connID)
	})

	l.server.ConnContext = func(ctx context.Context, a *frisbee.Async) context.Context {
		connID := a.RemoteAddr().String()

		l.wg.Add(1)
		go func() {
			defer l.wg.Done()

			if err := linkMessage(
				func(b []byte) error {
					pkg := packet.Get()

					pkg.Metadata.Operation = l.requestOperation
					pkg.Content.Write(b)
					pkg.Metadata.ContentLength = uint32(pkg.Content.Len())

					return a.WritePacket(pkg)
				},
				func(b []byte) error {
					pkg := packet.Get()

					pkg.Metadata.Operation = l.responseOperation
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

				marshal,
				unmarshal,
			); err != nil {
				onLinkError(ctx, a, err)
			}
		}()

		return context.WithValue(ctx, ConnIDKey, connID)
	}

	return l.server.StartWithListener(l.lis)
}

func (l *FrisbeeListener) Close() error {
	if l.server != nil {
		_ = l.server.Shutdown() // This also closes the underlying l.lis

		l.wg.Wait()

		l.server = nil
	}

	return nil
}
