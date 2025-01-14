package utils

import (
	"context"
	"strconv"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/require"
)

func TestBasicPublishAndReceive(t *testing.T) {
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
}

func TestReceiveAfterChannelFreed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	b := NewBroadcaster[string]()

	receive, err := b.Receive("test", ctx)
	require.NoError(t, err)

	b.Free("test", nil)

	_, err = receive()
	require.Equal(t, ErrClosed, err)
}

func TestReceiveAfterBroadcasterClosed(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	b := NewBroadcaster[string]()

	receive, err := b.Receive("test", ctx)
	require.NoError(t, err)

	b.Close(nil)

	_, err = receive()
	require.Equal(t, ErrClosed, err)
}

func TestReceiveWithCancelledContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	b := NewBroadcaster[string]()

	receive, err := b.Receive("test", ctx)
	require.NoError(t, err)

	cancel()

	_, err = receive()
	require.ErrorIs(t, err, context.Canceled)

	b.Free("test", nil)
}

func TestPublishAndReceiveWithMultipleChannels(t *testing.T) {
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
}

func TestPublishToNonExistentChannel(t *testing.T) {
	b := NewBroadcaster[string]()

	// Should not panic
	b.Publish("nonexistent", "test")
}

func TestConcurrentPublishAndReceive(t *testing.T) {
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
}

func TestCloseWhileReceiving(t *testing.T) {
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
}

func TestPublishToClosedBroadcaster(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := NewBroadcaster[string]()

	// Set up a channel and receiver
	receive, err := b.Receive("test", ctx)
	require.NoError(t, err)

	// Close the broadcaster
	b.Close(nil)

	// Try to publish after closing
	done := make(chan struct{})
	go func() {
		b.Publish("test", "hello")
		close(done)
	}()

	// The publish should return immediately since broadcaster is closed
	select {
	case <-done:
		// Success - publish returned
	case <-time.After(time.Second):
		t.Fatal("Publish to closed broadcaster blocked")
	}

	// Verify receiver gets closed error
	_, err = receive()
	require.Equal(t, ErrClosed, err)
}

func TestPublishToChannelWithDoneContext(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	b := NewBroadcaster[string]()

	// Set up a channel and receiver
	receive, err := b.Receive("test", ctx)
	require.NoError(t, err)

	// Cancel the context
	cancel()

	// Try to publish after context is cancelled
	done := make(chan struct{})
	go func() {
		b.Publish("test", "hello")
		close(done)
	}()

	// The publish should return quickly since context is done
	select {
	case <-done:
		// Success - publish returned
	case <-time.After(time.Second):
		t.Fatal("Publish to cancelled context blocked")
	}

	// Verify receiver gets context cancelled error
	_, err = receive()
	require.ErrorIs(t, err, context.Canceled)

	b.Free("test", nil)
}

func TestReceiveFromClosedBroadcaster(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	b := NewBroadcaster[string]()

	// Close the broadcaster first
	b.Close(nil)

	// Try to receive after closing
	_, err := b.Receive("test", ctx)
	require.Equal(t, ErrClosed, err)

	// Verify the channel wasn't created
	b.lock.Lock()
	_, exists := b.channels["test"]
	b.lock.Unlock()
	require.False(t, exists, "Channel should not be created when broadcaster is closed")
}
