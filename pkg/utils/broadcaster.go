package utils

import (
	"context"
	"net"
	"sync"
)

type channelWithContext[T any] struct {
	channel chan T

	ctx    context.Context
	cancel func()
}

type Broadcaster[T any] struct {
	channels     map[string]channelWithContext[T]
	channelsLock *sync.Mutex
}

func NewBroadcaster[T any]() *Broadcaster[T] {
	return &Broadcaster[T]{
		channels:     map[string]channelWithContext[T]{},
		channelsLock: &sync.Mutex{},
	}
}

func (b *Broadcaster[T]) Publish(channel string, v T) {
	b.channelsLock.Lock()
	c, ok := b.channels[channel]
	if !ok {
		b.channelsLock.Unlock()

		return
	}
	b.channelsLock.Unlock()

	select {
	case c.channel <- v:
		return

	case <-c.ctx.Done():
		return
	}
}

func (b *Broadcaster[T]) Receive(channel string, ctx context.Context) (*T, error) {
	b.channelsLock.Lock()
	c, ok := b.channels[channel]
	if !ok {
		ctx, cancel := context.WithCancel(ctx)
		c = channelWithContext[T]{
			channel: make(chan T),

			ctx:    ctx,
			cancel: cancel,
		}
		b.channels[channel] = c
	}
	b.channelsLock.Unlock()

	select {
	case v, ok := <-c.channel:
		if !ok {
			return nil, net.ErrClosed
		}
		return &v, nil

	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (b *Broadcaster[T]) Close(channel string) {
	b.channelsLock.Lock()
	c, ok := b.channels[channel]
	if ok {
		c.cancel()
		close(c.channel)
	}
	delete(b.channels, channel)
	b.channelsLock.Unlock()
}
