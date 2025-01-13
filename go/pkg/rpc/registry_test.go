package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

type nestedServiceLocal struct {
	value string
}

func (s *nestedServiceLocal) GetValue(ctx context.Context) (string, error) {
	return s.value, nil
}

func (s *nestedServiceLocal) SetValue(ctx context.Context, newValue string) error {
	s.value = newValue

	return nil
}

type nestedServiceRemote struct {
	GetValue func(ctx context.Context) (string, error)
	SetValue func(ctx context.Context, newValue string) error
}

type serverLocal struct {
	counter int64

	Nested *nestedServiceLocal

	ForRemotes func(cb func(remoteID string, remote clientRemote) error) error
}

func (s *serverLocal) TestSimple(ctx context.Context, delta int64) (int64, error) {
	return atomic.AddInt64(&s.counter, delta), nil
}

func (s *serverLocal) TestCallback(ctx context.Context, delta int64) (int64, error) {
	targetID := GetRemoteID(ctx)

	if err := s.ForRemotes(func(remoteID string, remote clientRemote) error {
		if remoteID == targetID {
			if err := remote.TestSimple(ctx, fmt.Sprintf("Incrementing counter by %v", delta)); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return -1, err
	}

	return atomic.AddInt64(&s.counter, delta), nil
}

type serverRemote struct {
	TestSimple   func(ctx context.Context, delta int64) (int64, error)
	TestCallback func(ctx context.Context, delta int64) (int64, error)

	Nested nestedServiceRemote
}

type clientLocal struct {
	counter int64

	messages        []string
	messagesLock    sync.Mutex
	messageReceived sync.WaitGroup

	Nested *nestedServiceLocal

	ForRemotes func(cb func(remoteID string, remote serverRemote) error) error
}

func (c *clientLocal) TestSimple(ctx context.Context, msg string) error {
	c.messagesLock.Lock()
	c.messages = append(c.messages, msg)
	c.messagesLock.Unlock()

	c.messageReceived.Done()

	return nil
}

func (s *clientLocal) TestCallback(ctx context.Context, delta int64) (int64, error) {
	targetID := GetRemoteID(ctx)

	if err := s.ForRemotes(func(remoteID string, remote serverRemote) error {
		if remoteID == targetID {
			if _, err := remote.TestSimple(ctx, 1); err != nil {
				return err
			}

		}
		return nil
	}); err != nil {
		return -1, err
	}

	return atomic.AddInt64(&s.counter, delta), nil
}

type clientRemote struct {
	TestSimple   func(ctx context.Context, msg string) error
	TestCallback func(ctx context.Context, delta int64) (int64, error)

	Nested nestedServiceRemote
}

type closureServerLocal struct {
	ForRemotes func(cb func(remoteID string, remote closureClientRemote) error) error
}

func (s *closureServerLocal) Iterate(
	ctx context.Context,
	length int,
	onIteration func(ctx context.Context, i int, b string) (string, error),
) (int, error) {
	for i := 0; i < length; i++ {
		_, err := onIteration(ctx, i, "This is from the callee")
		if err != nil {
			return -1, err
		}
	}
	return length, nil
}

type closureServerRemote struct {
	Iterate func(
		ctx context.Context,
		length int,
		onIteration func(ctx context.Context, i int, b string) (string, error),
	) (int, error)
}

type closureClientLocal struct {
	ForRemotes func(cb func(remoteID string, remote closureServerRemote) error) error
}

func (s *closureClientLocal) Iterate(
	ctx context.Context,
	length int,
	onIteration func(ctx context.Context, i int, b string) (string, error),
) (int, error) {
	for i := 0; i < length; i++ {
		_, err := onIteration(ctx, i, "This is from the callee")
		if err != nil {
			return -1, err
		}
	}
	return length, nil
}

type closureClientRemote struct {
	Iterate func(
		ctx context.Context,
		length int,
		onIteration func(ctx context.Context, i int, b string) (string, error),
	) (int, error)
}

func setupConnection(t *testing.T) (net.Listener, *sync.WaitGroup, *sync.WaitGroup) {
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	var serverConnected, clientConnected sync.WaitGroup
	serverConnected.Add(1)
	clientConnected.Add(1)

	return lis, &serverConnected, &clientConnected
}

func startServer[R, L any](t *testing.T, ctx context.Context, lis net.Listener, serverLocal L, serverConnected *sync.WaitGroup) (*Registry[R, json.RawMessage], *sync.WaitGroup) {
	serverRegistry := NewRegistry[R, json.RawMessage](
		serverLocal,
		&RegistryHooks{
			OnClientConnect: func(remoteID string) {
				serverConnected.Done()
			},
		},
	)

	var serverDone sync.WaitGroup
	serverDone.Add(1)

	go func() {
		defer serverDone.Done()

		conn, err := lis.Accept()
		require.NoError(t, err)

		defer conn.Close()

		linkCtx, cancelLinkCtx := context.WithCancel(ctx)
		defer cancelLinkCtx()

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		if err := serverRegistry.LinkStream(
			linkCtx,

			func(v Message[json.RawMessage]) error {
				return encoder.Encode(v)
			},
			func(v *Message[json.RawMessage]) error {
				return decoder.Decode(v)
			},

			func(v any) (json.RawMessage, error) {
				b, err := json.Marshal(v)
				if err != nil {
					return nil, err
				}
				return json.RawMessage(b), nil
			},
			func(data json.RawMessage, v any) error {
				return json.Unmarshal([]byte(data), v)
			},

			nil,
		); err != nil && !errors.Is(err, io.EOF) {
			select {
			case <-ctx.Done():
				return
			default:
			}

			require.NoError(t, err)
		}
	}()

	return serverRegistry, &serverDone
}

func startClient[R, L any](t *testing.T, ctx context.Context, addr string, clientLocal L, clientConnected *sync.WaitGroup) (*Registry[R, json.RawMessage], *sync.WaitGroup) {
	clientRegistry := NewRegistry[R, json.RawMessage](
		clientLocal,

		&RegistryHooks{
			OnClientConnect: func(remoteID string) {
				clientConnected.Done()
			},
		},
	)

	conn, err := net.Dial("tcp", addr)
	require.NoError(t, err)

	var clientDone sync.WaitGroup
	clientDone.Add(1)

	go func() {
		defer clientDone.Done()
		defer conn.Close()

		linkCtx, cancelLinkCtx := context.WithCancel(ctx)
		defer cancelLinkCtx()

		encoder := json.NewEncoder(conn)
		decoder := json.NewDecoder(conn)

		if err := clientRegistry.LinkStream(
			linkCtx,

			func(v Message[json.RawMessage]) error {
				return encoder.Encode(v)
			},
			func(v *Message[json.RawMessage]) error {
				return decoder.Decode(v)
			},

			func(v any) (json.RawMessage, error) {
				b, err := json.Marshal(v)
				if err != nil {
					return nil, err
				}
				return json.RawMessage(b), nil
			},

			func(data json.RawMessage, v any) error {
				return json.Unmarshal([]byte(data), v)
			},

			nil,
		); err != nil {
			select {
			case <-ctx.Done():
				return
			default:
			}

			require.NoError(t, err)
		}
	}()

	return clientRegistry, &clientDone
}

func TestSimpleRPCFromClientToServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lis, serverConnected, clientConnected := setupConnection(t)
	defer lis.Close()

	_, serverDone := startServer[clientRemote, *serverLocal](t, ctx, lis, &serverLocal{}, serverConnected)
	clientRegistry, clientDone := startClient[serverRemote, *clientLocal](t, ctx, lis.Addr().String(), &clientLocal{}, clientConnected)

	// Wait for client to connect to server
	serverConnected.Wait()
	clientConnected.Wait()

	// Test client calling server
	err := clientRegistry.ForRemotes(func(remoteID string, remote serverRemote) error {
		count, err := remote.TestSimple(ctx, 1)
		require.NoError(t, err)
		require.Equal(t, int64(1), count)

		count, err = remote.TestSimple(ctx, 2)
		require.NoError(t, err)
		require.Equal(t, int64(3), count)

		return nil
	})
	require.NoError(t, err)

	cancel()
	clientDone.Wait()
	serverDone.Wait()
}

func TestSimpleRPCFromServerToClient(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lis, serverConnected, clientConnected := setupConnection(t)
	defer lis.Close()

	cl := &clientLocal{}
	cl.messageReceived.Add(1) // Expect one message

	serverRegistry, serverDone := startServer[clientRemote, *serverLocal](t, ctx, lis, &serverLocal{}, serverConnected)
	_, clientDone := startClient[serverRemote, *clientLocal](t, ctx, lis.Addr().String(), cl, clientConnected)

	// Wait for client to connect to server
	serverConnected.Wait()
	clientConnected.Wait()

	// Test server calling client
	err := serverRegistry.ForRemotes(func(remoteID string, remote clientRemote) error {
		err := remote.TestSimple(ctx, "test message")
		require.NoError(t, err)
		return nil
	})
	require.NoError(t, err)

	// Wait for message to be received
	cl.messageReceived.Wait()

	cancel()
	clientDone.Wait()
	serverDone.Wait()

	// Verify results
	require.Len(t, cl.messages, 1)
	require.Equal(t, "test message", cl.messages[0])
}

func TestServerRPCWithCallbackToClient(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lis, serverConnected, clientConnected := setupConnection(t)
	defer lis.Close()

	cl := &clientLocal{}
	cl.messageReceived.Add(1) // Expect one message

	sl := &serverLocal{}

	serverRegistry, serverDone := startServer[clientRemote, *serverLocal](t, ctx, lis, sl, serverConnected)
	clientRegistry, clientDone := startClient[serverRemote, *clientLocal](t, ctx, lis.Addr().String(), cl, clientConnected)

	cl.ForRemotes = clientRegistry.ForRemotes
	sl.ForRemotes = serverRegistry.ForRemotes

	// Wait for client to connect to server
	serverConnected.Wait()
	clientConnected.Wait()

	iterations := 5
	err := clientRegistry.ForRemotes(func(remoteID string, remote serverRemote) error {
		length, err := remote.TestCallback(ctx, int64(iterations))
		require.NoError(t, err)
		require.Equal(t, length, int64(iterations))
		return nil
	})
	require.NoError(t, err)

	// Wait for message to be received
	cl.messageReceived.Wait()

	cancel()
	clientDone.Wait()
	serverDone.Wait()

	// Verify results
	require.Len(t, cl.messages, 1)
	require.Equal(t, fmt.Sprintf("Incrementing counter by %v", iterations), cl.messages[0])
}

func TestClientRPCWithCallbackToServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lis, serverConnected, clientConnected := setupConnection(t)
	defer lis.Close()

	cl := &clientLocal{}
	sl := &serverLocal{}

	serverRegistry, serverDone := startServer[clientRemote, *serverLocal](t, ctx, lis, sl, serverConnected)
	clientRegistry, clientDone := startClient[serverRemote, *clientLocal](t, ctx, lis.Addr().String(), cl, clientConnected)

	sl.ForRemotes = serverRegistry.ForRemotes
	cl.ForRemotes = clientRegistry.ForRemotes

	// Wait for client to connect to server
	serverConnected.Wait()
	clientConnected.Wait()

	iterations := 5
	err := serverRegistry.ForRemotes(func(remoteID string, remote clientRemote) error {
		length, err := remote.TestCallback(ctx, int64(iterations))
		require.NoError(t, err)
		require.Equal(t, length, int64(iterations))
		return nil
	})
	require.NoError(t, err)

	cancel()
	clientDone.Wait()
	serverDone.Wait()

	// Verify results
	require.Equal(t, int64(1), sl.counter)
}

func TestConcurrentRPCCalls(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lis, serverConnected, clientConnected := setupConnection(t)
	defer lis.Close()

	cl := &clientLocal{}
	sl := &serverLocal{}

	serverRegistry, serverDone := startServer[clientRemote, *serverLocal](t, ctx, lis, sl, serverConnected)
	clientRegistry, clientDone := startClient[serverRemote, *clientLocal](t, ctx, lis.Addr().String(), cl, clientConnected)

	cl.ForRemotes = clientRegistry.ForRemotes
	sl.ForRemotes = serverRegistry.ForRemotes

	// Wait for client to connect to server
	serverConnected.Wait()
	clientConnected.Wait()

	iterations := 5
	err := serverRegistry.ForRemotes(func(remoteID string, remote clientRemote) error {
		length, err := remote.TestCallback(ctx, int64(iterations))
		require.NoError(t, err)
		require.Equal(t, length, int64(iterations))

		// Concurrent client calling server
		clientRegistry.ForRemotes(func(remoteID string, remote serverRemote) error {
			if _, err := remote.TestSimple(ctx, 3); err != nil {
				return err
			}
			return nil
		})

		return nil
	})
	require.NoError(t, err)

	cancel()
	clientDone.Wait()
	serverDone.Wait()

	// Verify results
	require.Equal(t, int64(4), sl.counter)
}

func TestNestedRPCFromClientToServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lis, serverConnected, clientConnected := setupConnection(t)
	defer lis.Close()

	cl := &clientLocal{
		Nested: &nestedServiceLocal{},
	}
	sl := &serverLocal{
		Nested: &nestedServiceLocal{},
	}

	_, serverDone := startServer[clientRemote, *serverLocal](t, ctx, lis, sl, serverConnected)
	clientRegistry, clientDone := startClient[serverRemote, *clientLocal](t, ctx, lis.Addr().String(), cl, clientConnected)

	cl.ForRemotes = clientRegistry.ForRemotes

	// Wait for client to connect to server
	serverConnected.Wait()
	clientConnected.Wait()

	err := clientRegistry.ForRemotes(func(remoteID string, remote serverRemote) error {
		err := remote.Nested.SetValue(ctx, "test value")
		require.NoError(t, err)

		value, err := remote.Nested.GetValue(ctx)
		require.NoError(t, err)
		require.Equal(t, "test value", value)

		err = remote.Nested.SetValue(ctx, "updated value")
		require.NoError(t, err)

		value, err = remote.Nested.GetValue(ctx)
		require.NoError(t, err)
		require.Equal(t, "updated value", value)

		return nil
	})
	require.NoError(t, err)

	cancel()
	clientDone.Wait()
	serverDone.Wait()
}

func TestBidirectionalNestedRPC(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lis, serverConnected, clientConnected := setupConnection(t)
	defer lis.Close()

	cl := &clientLocal{
		Nested: &nestedServiceLocal{
			value: "initial client value",
		},
	}
	sl := &serverLocal{
		Nested: &nestedServiceLocal{},
	}

	serverRegistry, serverDone := startServer[clientRemote, *serverLocal](t, ctx, lis, sl, serverConnected)
	clientRegistry, clientDone := startClient[serverRemote, *clientLocal](t, ctx, lis.Addr().String(), cl, clientConnected)

	cl.ForRemotes = clientRegistry.ForRemotes
	sl.ForRemotes = serverRegistry.ForRemotes

	// Wait for client to connect to server
	serverConnected.Wait()
	clientConnected.Wait()

	// Test client calling server's nested service
	err := clientRegistry.ForRemotes(func(remoteID string, remote serverRemote) error {
		err := remote.Nested.SetValue(ctx, "test server value")
		require.NoError(t, err)

		value, err := remote.Nested.GetValue(ctx)
		require.NoError(t, err)
		require.Equal(t, "test server value", value)

		return nil
	})
	require.NoError(t, err)

	// Test server calling client's nested service
	err = serverRegistry.ForRemotes(func(remoteID string, remote clientRemote) error {
		err := remote.Nested.SetValue(ctx, "test client value")
		require.NoError(t, err)

		value, err := remote.Nested.GetValue(ctx)
		require.NoError(t, err)
		require.Equal(t, "test client value", value)

		// Test concurrent access to both nested services
		var wg sync.WaitGroup
		wg.Add(2)

		go func() {
			defer wg.Done()
			value, err := remote.Nested.GetValue(ctx)
			require.NoError(t, err)
			require.Equal(t, "test client value", value)
		}()

		go func() {
			defer wg.Done()
			err := clientRegistry.ForRemotes(func(remoteID string, remote serverRemote) error {
				value, err := remote.Nested.GetValue(ctx)
				require.NoError(t, err)
				require.Equal(t, "test server value", value)
				return nil
			})
			require.NoError(t, err)
		}()

		wg.Wait()
		return nil
	})
	require.NoError(t, err)

	// Test nested service state persistence
	err = clientRegistry.ForRemotes(func(remoteID string, remote serverRemote) error {
		value, err := remote.Nested.GetValue(ctx)
		require.NoError(t, err)
		require.Equal(t, "test server value", value)
		return nil
	})
	require.NoError(t, err)

	err = serverRegistry.ForRemotes(func(remoteID string, remote clientRemote) error {
		value, err := remote.Nested.GetValue(ctx)
		require.NoError(t, err)
		require.Equal(t, "test client value", value)
		return nil
	})
	require.NoError(t, err)

	cancel()
	clientDone.Wait()
	serverDone.Wait()
}

func TestRPCWithClosureOnServerFromClient(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lis, serverConnected, clientConnected := setupConnection(t)
	defer lis.Close()

	cl := &closureClientLocal{}
	sl := &closureServerLocal{}

	_, serverDone := startServer[closureClientRemote, *closureServerLocal](t, ctx, lis, sl, serverConnected)
	clientRegistry, clientDone := startClient[closureServerRemote, *closureClientLocal](t, ctx, lis.Addr().String(), cl, clientConnected)

	cl.ForRemotes = clientRegistry.ForRemotes

	// Wait for client to connect to server
	serverConnected.Wait()
	clientConnected.Wait()

	// Test iterations
	expectedIterations := 5
	actualIterations := 0

	if err := clientRegistry.ForRemotes(func(remoteID string, remote closureServerRemote) error {
		count, err := remote.Iterate(ctx, expectedIterations, func(ctx context.Context, i int, b string) (string, error) {
			actualIterations++

			return "This is from the caller", nil
		})
		require.NoError(t, err)

		require.Equal(t, expectedIterations, count)
		return nil
	}); err != nil {
		require.NoError(t, err)
	}

	// Cleanup
	cancel()
	clientDone.Wait()
	serverDone.Wait()

	require.Equal(t, expectedIterations, actualIterations)
}

func TestRPCWithClosureOnClientFromServer(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	lis, serverConnected, clientConnected := setupConnection(t)
	defer lis.Close()

	cl := &closureClientLocal{}
	sl := &closureServerLocal{}

	serverRegistry, serverDone := startServer[closureClientRemote, *closureServerLocal](t, ctx, lis, sl, serverConnected)
	_, clientDone := startClient[closureServerRemote, *closureClientLocal](t, ctx, lis.Addr().String(), cl, clientConnected)

	sl.ForRemotes = serverRegistry.ForRemotes

	// Wait for client to connect to server
	serverConnected.Wait()
	clientConnected.Wait()

	// Test iterations
	expectedIterations := 5
	actualIterations := 0

	if err := serverRegistry.ForRemotes(func(remoteID string, remote closureClientRemote) error {
		count, err := remote.Iterate(ctx, expectedIterations, func(ctx context.Context, i int, b string) (string, error) {
			actualIterations++

			return "This is from the caller", nil
		})
		require.NoError(t, err)

		require.Equal(t, expectedIterations, count)
		return nil
	}); err != nil {
		require.NoError(t, err)
	}

	// Cleanup
	cancel()
	clientDone.Wait()
	serverDone.Wait()

	require.Equal(t, expectedIterations, actualIterations)
}
