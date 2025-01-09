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

type serverRemote struct {
	Println func(ctx context.Context, msg string) error
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

type clientRemote struct {
	Increment func(ctx context.Context, delta int64) (int64, error)
}

func TestRegistry(t *testing.T) {
	tests := []struct {
		name string
		run  func(t *testing.T)
	}{
		{
			name: "calling RPC on server from client and on client from server",
			run: func(t *testing.T) {
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()

				// Setup server
				lis, err := net.Listen("tcp", "localhost:0")
				require.NoError(t, err)
				defer lis.Close()

				var serverConnected, clientConnected sync.WaitGroup
				serverConnected.Add(1)
				clientConnected.Add(1)

				var serverDone sync.WaitGroup
				serverDone.Add(1)

				serverRegistry := NewRegistry[serverRemote, json.RawMessage](
					&serverLocal{},

					&RegistryHooks{
						OnClientConnect: func(remoteID string) {
							serverConnected.Done()
						},
					},
				)

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

				// Setup client
				clientLocal := &clientLocal{}
				clientLocal.messageReceived.Add(1) // Expect one message

				clientRegistry := NewRegistry[clientRemote, json.RawMessage](
					clientLocal,

					&RegistryHooks{
						OnClientConnect: func(remoteID string) {
							clientConnected.Done()
						},
					},
				)

				conn, err := net.Dial("tcp", lis.Addr().String())
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

				// Wait for client to connect to server
				serverConnected.Wait()
				clientConnected.Wait()

				// Call RPC on server from client
				err = clientRegistry.ForRemotes(func(remoteID string, remote clientRemote) error {
					count, err := remote.Increment(ctx, 1)
					require.NoError(t, err)
					require.Equal(t, int64(1), count)

					count, err = remote.Increment(ctx, 2)
					require.NoError(t, err)
					require.Equal(t, int64(3), count)

					return nil
				})
				require.NoError(t, err)

				// Call RPC on client from server
				err = serverRegistry.ForRemotes(func(remoteID string, remote serverRemote) error {
					err := remote.Println(ctx, "test message")
					require.NoError(t, err)

					return nil
				})
				require.NoError(t, err)

				// Wait for message to be received
				clientLocal.messageReceived.Wait()

				// Cancel context and wait for client and server goroutines to finish
				cancel()
				clientDone.Wait()
				serverDone.Wait()

				// Verify results
				require.Len(t, clientLocal.messages, 1)
				require.Equal(t, "test message", clientLocal.messages[0])
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}
