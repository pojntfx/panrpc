package utils

import (
	"context"
	"errors"
	"sync"
)

var (
	ErrClosed = errors.New("closed")
)

type channelWithContext[T any] struct {
	channel chan T

	ctx    context.Context
	cancel func(cause error)
}

type Broadcaster[T any] struct {
	channels map[string]channelWithContext[T]
	closed   bool
	lock     *sync.Mutex
}

func NewBroadcaster[T any]() *Broadcaster[T] {
	return &Broadcaster[T]{
		channels: map[string]channelWithContext[T]{},
		lock:     &sync.Mutex{},
	}
}

func (b *Broadcaster[T]) Publish(channel string, v T) {
	b.lock.Lock()
	if b.closed {
		b.lock.Unlock()

		return
	}

	c, ok := b.channels[channel]
	if !ok {
		b.lock.Unlock()

		return
	}
	b.lock.Unlock()

	select {
	case c.channel <- v:
		return

	case <-c.ctx.Done():
		return
	}
}

func (b *Broadcaster[T]) Receive(channel string, ctx context.Context) (func() (*T, error), error) {
	b.lock.Lock()
	if b.closed {
		b.lock.Unlock()

		return nil, ErrClosed
	}

	c, ok := b.channels[channel]
	if !ok {
		ctx, cancel := context.WithCancelCause(ctx)
		c = channelWithContext[T]{
			channel: make(chan T),

			ctx:    ctx,
			cancel: cancel,
		}
		b.channels[channel] = c
	}
	b.lock.Unlock()

	return func() (*T, error) {
		select {
		case v, ok := <-c.channel:
			if !ok {
				return nil, ErrClosed
			}
			return &v, nil

		case <-ctx.Done():
			return nil, ctx.Err()
		}
	}, nil
}

func (b *Broadcaster[T]) Free(channel string, err error) {
	b.lock.Lock()
	c, ok := b.channels[channel]
	if ok {
		c.cancel(err)
		close(c.channel)
	}
	delete(b.channels, channel)
	b.lock.Unlock()
}

func (b *Broadcaster[T]) Close(err error) {
	b.lock.Lock()
	for _, c := range b.channels {
		c.cancel(err)
		close(c.channel)
	}
	b.channels = map[string]channelWithContext[T]{}
	b.closed = true
	b.lock.Unlock()
}
