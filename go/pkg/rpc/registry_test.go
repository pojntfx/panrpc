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

func (s *serverLocal) Increment(ctx context.Context, delta int64) (int64, error) {
	return atomic.AddInt64(&s.counter, delta), nil
}

func (s *serverLocal) Iterate(ctx context.Context, delta int64) (int64, error) {
	targetID := GetRemoteID(ctx)

	if err := s.ForRemotes(func(remoteID string, remote clientRemote) error {
		if remoteID == targetID {
			if err := remote.Println(ctx, fmt.Sprintf("Incrementing counter by %v", delta)); err != nil {
				return err
			}
		}
		return nil
	}); err != nil {
		return -1, err
	}

	return atomic.AddInt64(&s.counter, delta), nil
}

func (s *serverLocal) ProcessBatch(
	ctx context.Context,
	items []string,
	onProcess func(ctx context.Context, item string, i int) (bool, error),
) (int, error) {
	processed := 0
	for i, item := range items {
		shouldContinue, err := onProcess(ctx, item, i)
		if err != nil {
			return processed, err
		}

		processed++

		if !shouldContinue {
			return processed, nil
		}
	}

	return processed, nil
}

type serverRemote struct {
	Increment    func(ctx context.Context, delta int64) (int64, error)
	Iterate      func(ctx context.Context, delta int64) (int64, error)
	ProcessBatch func(
		ctx context.Context,
		items []string,
		onProcess func(ctx context.Context, item string, i int) (bool, error),
	) (int, error)

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

func (c *clientLocal) Println(ctx context.Context, msg string) error {
	c.messagesLock.Lock()
	c.messages = append(c.messages, msg)
	c.messagesLock.Unlock()

	c.messageReceived.Done()

	return nil
}

func (s *clientLocal) Iterate(ctx context.Context, delta int64) (int64, error) {
	targetID := GetRemoteID(ctx)

	if err := s.ForRemotes(func(remoteID string, remote serverRemote) error {
		if remoteID == targetID {
			if _, err := remote.Increment(ctx, 1); err != nil {
				return err
			}

		}
		return nil
	}); err != nil {
		return -1, err
	}

	return atomic.AddInt64(&s.counter, delta), nil
}

func (c *clientLocal) ProcessBatch(
	ctx context.Context,
	items []string,
	onProcess func(ctx context.Context, item string, i int) (bool, error),
) (int, error) {
	processed := 0
	for i, item := range items {
		shouldContinue, err := onProcess(ctx, item, i)
		if err != nil {
			return processed, err
		}

		processed++

		if !shouldContinue {
			return processed, nil
		}
	}

	return processed, nil
}

type clientRemote struct {
	Println      func(ctx context.Context, msg string) error
	Iterate      func(ctx context.Context, delta int64) (int64, error)
	ProcessBatch func(
		ctx context.Context,
		items []string,
		onProcess func(ctx context.Context, item string, i int) (bool, error),
	) (int, error)

	Nested nestedServiceRemote
}

func setupConnection(t *testing.T) (net.Listener, *sync.WaitGroup, *sync.WaitGroup) {
	lis, err := net.Listen("tcp", "localhost:0")
	require.NoError(t, err)

	var serverConnected, clientConnected sync.WaitGroup
	serverConnected.Add(1)
	clientConnected.Add(1)

	return lis, &serverConnected, &clientConnected
}

func startServer(t *testing.T, ctx context.Context, lis net.Listener, serverLocal *serverLocal, serverConnected *sync.WaitGroup) (*Registry[clientRemote, json.RawMessage], *sync.WaitGroup) {
	serverRegistry := NewRegistry[clientRemote, json.RawMessage](
		serverLocal,
		&RegistryHooks{
			OnClientConnect: func(remoteID string) {
				serverConnected.Done()
			},
		},
	)
	serverLocal.ForRemotes = serverRegistry.ForRemotes

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

func startClient(t *testing.T, ctx context.Context, addr string, clientLocal *clientLocal, clientConnected *sync.WaitGroup) (*Registry[serverRemote, json.RawMessage], *sync.WaitGroup) {
	clientRegistry := NewRegistry[serverRemote, json.RawMessage](
		clientLocal,

		&RegistryHooks{
			OnClientConnect: func(remoteID string) {
				clientConnected.Done()
			},
		},
	)
	clientLocal.ForRemotes = clientRegistry.ForRemotes

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

func TestRegistry(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "calling RPC on server from client",
			run: func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				lis, serverConnected, clientConnected := setupConnection(t)
				defer lis.Close()

				_, serverDone := startServer(t, ctx, lis, &serverLocal{}, serverConnected)
				clientRegistry, clientDone := startClient(t, ctx, lis.Addr().String(), &clientLocal{}, clientConnected)

				// Wait for client to connect to server
				serverConnected.Wait()
				clientConnected.Wait()

				// Test client calling server
				err := clientRegistry.ForRemotes(func(remoteID string, remote serverRemote) error {
					count, err := remote.Increment(ctx, 1)
					require.NoError(t, err)
					require.Equal(t, int64(1), count)

					count, err = remote.Increment(ctx, 2)
					require.NoError(t, err)
					require.Equal(t, int64(3), count)

					return nil
				})
				require.NoError(t, err)

				cancel()
				clientDone.Wait()
				serverDone.Wait()
			},
		},
		{
			name: "calling RPC on client from server",
			run: func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				lis, serverConnected, clientConnected := setupConnection(t)
				defer lis.Close()

				clientLocal := &clientLocal{}
				clientLocal.messageReceived.Add(1) // Expect one message

				serverRegistry, serverDone := startServer(t, ctx, lis, &serverLocal{}, serverConnected)
				_, clientDone := startClient(t, ctx, lis.Addr().String(), clientLocal, clientConnected)

				// Wait for client to connect to server
				serverConnected.Wait()
				clientConnected.Wait()

				// Test server calling client
				err := serverRegistry.ForRemotes(func(remoteID string, remote clientRemote) error {
					err := remote.Println(ctx, "test message")
					require.NoError(t, err)
					return nil
				})
				require.NoError(t, err)

				// Wait for message to be received
				clientLocal.messageReceived.Wait()

				cancel()
				clientDone.Wait()
				serverDone.Wait()

				// Verify results
				require.Len(t, clientLocal.messages, 1)
				require.Equal(t, "test message", clientLocal.messages[0])
			},
		},
		{
			name: "callback from client to server",
			run: func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				lis, serverConnected, clientConnected := setupConnection(t)
				defer lis.Close()

				clientLocal := &clientLocal{}
				clientLocal.messageReceived.Add(1) // Expect one message

				_, serverDone := startServer(t, ctx, lis, &serverLocal{}, serverConnected)
				clientRegistry, clientDone := startClient(t, ctx, lis.Addr().String(), clientLocal, clientConnected)

				// Wait for client to connect to server
				serverConnected.Wait()
				clientConnected.Wait()

				// Track callback invocations
				iterations := 5

				// Test client calling server with callback
				err := clientRegistry.ForRemotes(func(remoteID string, remote serverRemote) error {
					length, err := remote.Iterate(ctx, int64(iterations))
					require.NoError(t, err)

					require.Equal(t, length, int64(iterations))

					return nil
				})
				require.NoError(t, err)

				// Wait for message to be received
				clientLocal.messageReceived.Wait()

				cancel()
				clientDone.Wait()
				serverDone.Wait()

				// Verify results
				require.Len(t, clientLocal.messages, 1)
				require.Equal(t, fmt.Sprintf("Incrementing counter by %v", iterations), clientLocal.messages[0])
			},
		},
		{
			name: "callback from server to client",
			run: func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				lis, serverConnected, clientConnected := setupConnection(t)
				defer lis.Close()

				clientLocal := &clientLocal{}

				serverLocal := &serverLocal{}

				serverRegistry, serverDone := startServer(t, ctx, lis, serverLocal, serverConnected)
				_, clientDone := startClient(t, ctx, lis.Addr().String(), clientLocal, clientConnected)

				// Wait for client to connect to server
				serverConnected.Wait()
				clientConnected.Wait()

				// Track callback invocations
				iterations := 5

				// Test client calling server with callback
				err := serverRegistry.ForRemotes(func(remoteID string, remote clientRemote) error {
					length, err := remote.Iterate(ctx, int64(iterations))
					require.NoError(t, err)

					require.Equal(t, length, int64(iterations))

					return nil
				})
				require.NoError(t, err)

				cancel()
				clientDone.Wait()
				serverDone.Wait()

				// Verify results
				require.Equal(t, int64(1), serverLocal.counter)
			},
		},
		{
			name: "nested callbacks",
			run: func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				lis, serverConnected, clientConnected := setupConnection(t)
				defer lis.Close()

				clientLocal := &clientLocal{}

				serverLocal := &serverLocal{}

				serverRegistry, serverDone := startServer(t, ctx, lis, serverLocal, serverConnected)
				clientRegistry, clientDone := startClient(t, ctx, lis.Addr().String(), clientLocal, clientConnected)

				// Wait for client to connect to server
				serverConnected.Wait()
				clientConnected.Wait()

				// Track callback invocations
				iterations := 5

				// Test server calling server with callback
				err := serverRegistry.ForRemotes(func(remoteID string, remote clientRemote) error {
					length, err := remote.Iterate(ctx, int64(iterations))
					require.NoError(t, err)

					require.Equal(t, length, int64(iterations))

					// Test client calling server with callback
					clientRegistry.ForRemotes(func(remoteID string, remote serverRemote) error {
						if _, err := remote.Increment(ctx, 3); err != nil {
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
				require.Equal(t, int64(4), serverLocal.counter)
			},
		},
		{
			name: "nested RPCs",
			run: func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				lis, serverConnected, clientConnected := setupConnection(t)
				defer lis.Close()

				clientLocal := &clientLocal{
					Nested: &nestedServiceLocal{},
				}

				serverLocal := &serverLocal{
					Nested: &nestedServiceLocal{},
				}

				_, serverDone := startServer(t, ctx, lis, serverLocal, serverConnected)
				clientRegistry, clientDone := startClient(t, ctx, lis.Addr().String(), clientLocal, clientConnected)

				// Wait for client to connect to server
				serverConnected.Wait()
				clientConnected.Wait()

				// Test nested service operations
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
			},
		},
		{
			name: "bidirectional nested RPCs",
			run: func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				lis, serverConnected, clientConnected := setupConnection(t)
				defer lis.Close()

				// Initialize client with nested service
				clientLocal := &clientLocal{
					Nested: &nestedServiceLocal{
						value: "initial client value",
					},
				}

				serverLocal := &serverLocal{
					Nested: &nestedServiceLocal{},
				}

				serverRegistry, serverDone := startServer(t, ctx, lis, serverLocal, serverConnected)
				clientRegistry, clientDone := startClient(t, ctx, lis.Addr().String(), clientLocal, clientConnected)

				// Wait for client to connect to server
				serverConnected.Wait()
				clientConnected.Wait()

				// Test client calling server's nested service
				err := clientRegistry.ForRemotes(func(remoteID string, remote serverRemote) error {
					// Set and verify server value
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
					// Set and verify client value
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
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}
