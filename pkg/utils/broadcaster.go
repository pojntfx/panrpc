package utils

import (
	"sync"
	"time"
)

type Broadcaster[T any] struct {
	channels     map[string]chan T
	channelsLock *sync.Mutex
}

func NewBroadcaster[T any]() *Broadcaster[T] {
	return &Broadcaster[T]{
		channels:     map[string]chan T{},
		channelsLock: &sync.Mutex{},
	}
}

func (b *Broadcaster[T]) Publish(channel string, v T, timeout time.Duration) bool {
	b.channelsLock.Lock()
	c, ok := b.channels[channel]
	if !ok {
		c = make(chan T)
		b.channels[channel] = c
	}
	b.channelsLock.Unlock()

	select {
	case c <- v:
		return true

	case <-time.After(timeout):
		return false
	}
}

func (b *Broadcaster[T]) Receive(channel string, timeout time.Duration) (*T, bool) {
	b.channelsLock.Lock()
	c, ok := b.channels[channel]
	if !ok {
		c = make(chan T)
		b.channels[channel] = c
	}
	b.channelsLock.Unlock()

	select {
	case v, ok := <-c:
		if !ok {
			return nil, false
		}
		return &v, true

	case <-time.After(timeout):
		return nil, false
	}
}

func (b *Broadcaster[T]) Close(channel string) {
	b.channelsLock.Lock()
	c, ok := b.channels[channel]
	if ok {
		close(c)
	}
	delete(b.channels, channel)
	b.channelsLock.Unlock()
}
