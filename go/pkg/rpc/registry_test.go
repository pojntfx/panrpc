package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
)

type serverLocal struct {
	counter int64
}

func (s *serverLocal) Increment(ctx context.Context, delta int64) (int64, error) {
	return atomic.AddInt64(&s.counter, delta), nil
}

func (s *serverLocal) Iterate(
	ctx context.Context,
	length int,
	onIteration func(ctx context.Context, i int, b string) (string, error),
) (int, error) {
	for i := 0; i < length; i++ {
		if _, err := onIteration(ctx, i, "This is from the server"); err != nil {
			return -1, err
		}
	}

	return length, nil
}

type serverRemote struct {
	Println func(ctx context.Context, msg string) error
	Iterate func(
		ctx context.Context,
		length int,
		onIteration func(ctx context.Context, i int, b string) (string, error),
	) (int, error)
}

type clientLocal struct {
	messages        []string
	messagesLock    sync.Mutex
	messageReceived sync.WaitGroup
}

func (c *clientLocal) Println(ctx context.Context, msg string) error {
	c.messagesLock.Lock()
	c.messages = append(c.messages, msg)
	c.messagesLock.Unlock()

	c.messageReceived.Done()

	return nil
}

func (c *clientLocal) Iterate(
	ctx context.Context,
	length int,
	onIteration func(ctx context.Context, i int, b string) (string, error),
) (int, error) {
	for i := 0; i < length; i++ {
		if _, err := onIteration(ctx, i, "This is from the client"); err != nil {
			return -1, err
		}
	}

	return length, nil
}

type clientRemote struct {
	Increment func(ctx context.Context, delta int64) (int64, error)
	Iterate   func(
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

func startServer(t *testing.T, ctx context.Context, lis net.Listener, serverConnected *sync.WaitGroup) (*Registry[serverRemote, json.RawMessage], *sync.WaitGroup) {
	serverRegistry := NewRegistry[serverRemote, json.RawMessage](
		&serverLocal{},

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

func startClient(t *testing.T, ctx context.Context, addr string, clientLocal *clientLocal, clientConnected *sync.WaitGroup) (*Registry[clientRemote, json.RawMessage], *sync.WaitGroup) {
	clientRegistry := NewRegistry[clientRemote, json.RawMessage](
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

				_, serverDone := startServer(t, ctx, lis, serverConnected)
				clientRegistry, clientDone := startClient(t, ctx, lis.Addr().String(), &clientLocal{}, clientConnected)

				// Wait for client to connect to server
				serverConnected.Wait()
				clientConnected.Wait()

				// Test client calling server
				err := clientRegistry.ForRemotes(func(remoteID string, remote clientRemote) error {
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

				serverRegistry, serverDone := startServer(t, ctx, lis, serverConnected)
				_, clientDone := startClient(t, ctx, lis.Addr().String(), clientLocal, clientConnected)

				// Wait for client to connect to server
				serverConnected.Wait()
				clientConnected.Wait()

				// Test server calling client
				err := serverRegistry.ForRemotes(func(remoteID string, remote serverRemote) error {
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
			name: "callback from server to client",
			run: func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				lis, serverConnected, clientConnected := setupConnection(t)
				defer lis.Close()

				clientLocal := &clientLocal{}
				serverRegistry, serverDone := startServer(t, ctx, lis, serverConnected)
				_, clientDone := startClient(t, ctx, lis.Addr().String(), clientLocal, clientConnected)

				// Wait for client to connect to server
				serverConnected.Wait()
				clientConnected.Wait()

				// Track callback invocations
				callbackCount := 0
				expectedLength := 5

				// Test server calling client with callback
				err := serverRegistry.ForRemotes(func(remoteID string, remote serverRemote) error {
					length, err := remote.Iterate(ctx, expectedLength, func(ctx context.Context, i int, b string) (string, error) {
						callbackCount++

						require.Equal(t, "This is from the client", b)

						return "response from server", nil
					})
					require.NoError(t, err)

					require.Equal(t, expectedLength, length)

					return nil
				})
				require.NoError(t, err)

				// Verify callback was called expected number of times
				require.Equal(t, expectedLength, callbackCount)

				cancel()
				clientDone.Wait()
				serverDone.Wait()
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
				_, serverDone := startServer(t, ctx, lis, serverConnected)
				clientRegistry, clientDone := startClient(t, ctx, lis.Addr().String(), clientLocal, clientConnected)

				// Wait for client to connect to server
				serverConnected.Wait()
				clientConnected.Wait()

				// Track callback invocations
				callbackCount := 0
				expectedLength := 5

				// Test client calling server with callback
				err := clientRegistry.ForRemotes(func(remoteID string, remote clientRemote) error {
					length, err := remote.Iterate(ctx, expectedLength, func(ctx context.Context, i int, b string) (string, error) {
						callbackCount++

						require.Equal(t, "This is from the server", b)

						return "response from client", nil
					})
					require.NoError(t, err)

					require.Equal(t, expectedLength, length)

					return nil
				})
				require.NoError(t, err)

				// Verify callback was called expected number of times
				require.Equal(t, expectedLength, callbackCount)

				cancel()
				clientDone.Wait()
				serverDone.Wait()
			},
		},
		{
			name: "nested callbacks between client and server",
			run: func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				lis, serverConnected, clientConnected := setupConnection(t)
				defer lis.Close()

				clientLocal := &clientLocal{}
				serverRegistry, serverDone := startServer(t, ctx, lis, serverConnected)
				clientRegistry, clientDone := startClient(t, ctx, lis.Addr().String(), clientLocal, clientConnected)

				// Wait for client to connect to server
				serverConnected.Wait()
				clientConnected.Wait()

				// Track nested callback invocations
				outerCount := 0
				innerCount := 0
				expectedOuterLength := 3
				expectedInnerLength := 2

				// Test nested callbacks
				err := clientRegistry.ForRemotes(func(remoteID string, remote clientRemote) error {
					length, err := remote.Iterate(ctx, expectedOuterLength, func(ctx context.Context, i int, b string) (string, error) {
						outerCount++
						require.Equal(t, "This is from the server", b)

						// Make nested callback from server back to client
						var nestedErr error
						err := serverRegistry.ForRemotes(func(remoteID string, remote serverRemote) error {
							nestedLength, err := remote.Iterate(ctx, expectedInnerLength, func(ctx context.Context, i int, b string) (string, error) {
								innerCount++

								require.Equal(t, "This is from the client", b)

								return "response from nested callback", nil
							})
							if err != nil {
								nestedErr = err

								return err
							}

							require.Equal(t, expectedInnerLength, nestedLength)

							return nil
						})
						if nestedErr != nil {
							return "", nestedErr
						}

						return "response from outer callback", err
					})
					require.NoError(t, err)

					require.Equal(t, expectedOuterLength, length)

					return nil
				})
				require.NoError(t, err)

				// Verify callback counts
				require.Equal(t, expectedOuterLength, outerCount)
				require.Equal(t, expectedOuterLength*expectedInnerLength, innerCount)

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
