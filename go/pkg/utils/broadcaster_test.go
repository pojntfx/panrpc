package utils

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBroadcaster(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "basic publish and receive",
			run: func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				b := NewBroadcaster[string]()

				receive, err := b.Receive("test", ctx)
				require.NoError(t, err)

				go b.Publish("test", "hello")

				result, err := receive()
				require.NoError(t, err)
				require.Equal(t, "hello", *result)

				b.Free("test", nil)
			},
		},
		{
			name: "receive after channel freed",
			run: func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				b := NewBroadcaster[string]()

				receive, err := b.Receive("test", ctx)
				require.NoError(t, err)

				b.Free("test", nil)

				_, err = receive()
				require.Equal(t, ErrClosed, err)
			},
		},
		{
			name: "receive after broadcaster closed",
			run: func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				b := NewBroadcaster[string]()

				receive, err := b.Receive("test", ctx)
				require.NoError(t, err)

				b.Close(nil)

				_, err = receive()
				require.Equal(t, ErrClosed, err)
			},
		},
		{
			name: "receive with cancelled context",
			run: func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				b := NewBroadcaster[string]()

				receive, err := b.Receive("test", ctx)
				require.NoError(t, err)

				cancel()

				_, err = receive()
				require.ErrorIs(t, err, context.Canceled)

				b.Free("test", nil)
			},
		},
		{
			name: "publish and receive with multiple channels",
			run: func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				b := NewBroadcaster[string]()

				// Setup two channels
				receive1, err := b.Receive("ch1", ctx)
				require.NoError(t, err)
				receive2, err := b.Receive("ch2", ctx)
				require.NoError(t, err)

				// Publish to both channels
				var wg sync.WaitGroup
				wg.Add(2)

				go func() {
					b.Publish("ch1", "hello")
					wg.Done()
				}()

				go func() {
					b.Publish("ch2", "world")
					wg.Done()
				}()

				// Receive from both channels
				result1, err := receive1()
				require.NoError(t, err)
				require.Equal(t, "hello", *result1)

				result2, err := receive2()
				require.NoError(t, err)
				require.Equal(t, "world", *result2)

				wg.Wait()

				b.Free("ch1", nil)
				b.Free("ch2", nil)
			},
		},
		{
			name: "publish to non-existent channel",
			run: func(t *testing.T) {
				b := NewBroadcaster[string]()

				// Should not panic
				b.Publish("nonexistent", "test")
			},
		},
		{
			name: "concurrent publish and receive operations",
			run: func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				b := NewBroadcaster[int]()

				const numChannels = 3
				const messagesPerChannel = 5

				var wg sync.WaitGroup
				wg.Add(numChannels * 2) // Publishers and receivers

				for i := 0; i < numChannels; i++ {
					channel := strconv.Itoa(i)

					receive, err := b.Receive(channel, ctx)
					require.NoError(t, err)

					// Start publisher
					go func() {
						defer wg.Done()
						for j := 0; j < messagesPerChannel; j++ {
							b.Publish(channel, j)
						}
					}()

					// Start receiver
					go func() {
						defer wg.Done()
						for j := 0; j < messagesPerChannel; j++ {
							result, err := receive()
							if err != nil {
								t.Errorf("receive error on channel %s: %v", channel, err)
								return
							}
							require.NotNil(t, result)
							require.GreaterOrEqual(t, *result, 0)
							require.Less(t, *result, messagesPerChannel)
						}
					}()
				}

				wg.Wait()

				for i := 0; i < numChannels; i++ {
					b.Free(string(rune('A'+i)), nil)
				}
			},
		},
		{
			name: "close while receiving",
			run: func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				b := NewBroadcaster[string]()

				receive, err := b.Receive("test", ctx)
				require.NoError(t, err)

				done := make(chan struct{})
				go func() {
					_, err := receive()
					require.Equal(t, ErrClosed, err)
					close(done)
				}()

				// Give the goroutine time to start receiving
				time.Sleep(time.Millisecond * 10)
				b.Close(nil)

				select {
				case <-done:
				case <-time.After(time.Second * 10):
					t.Fatal("test timed out")
				}
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}
