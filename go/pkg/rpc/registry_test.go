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

func startServer(t *testing.T, ctx context.Context, lis net.Listener, serverConnected *sync.WaitGroup) (*Registry[clientRemote, json.RawMessage], *sync.WaitGroup) {
	serverLocal := &serverLocal{
		Nested: &nestedServiceLocal{},
	}
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

				_, serverDone := startServer(t, ctx, lis, serverConnected)
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

				serverRegistry, serverDone := startServer(t, ctx, lis, serverConnected)
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
		// {
		// 	name: "callback from server to client",
		// 	run: func(t *testing.T) {
		// 		ctx, cancel := context.WithCancel(context.Background())
		// 		defer cancel()

		// 		lis, serverConnected, clientConnected := setupConnection(t)
		// 		defer lis.Close()

		// 		clientLocal := &clientLocal{}
		// 		serverRegistry, serverDone := startServer(t, ctx, lis, serverConnected)
		// 		_, clientDone := startClient(t, ctx, lis.Addr().String(), clientLocal, clientConnected)

		// 		// Wait for client to connect to server
		// 		serverConnected.Wait()
		// 		clientConnected.Wait()

		// 		// Track callback invocations
		// 		callbackCount := 0
		// 		expectedLength := 5

		// 		// Test server calling client with callback
		// 		err := serverRegistry.ForRemotes(func(remoteID string, remote serverRemote) error {
		// 			length, err := remote.Iterate(ctx, expectedLength, func(ctx context.Context, i int, b string) (string, error) {
		// 				callbackCount++

		// 				require.Equal(t, "This is from the client", b)

		// 				return "response from server", nil
		// 			})
		// 			require.NoError(t, err)

		// 			require.Equal(t, expectedLength, length)

		// 			return nil
		// 		})
		// 		require.NoError(t, err)

		// 		// Verify callback was called expected number of times
		// 		require.Equal(t, expectedLength, callbackCount)

		// 		cancel()
		// 		clientDone.Wait()
		// 		serverDone.Wait()
		// 	},
		// },
		// {
		// 	name: "callback from client to server",
		// 	run: func(t *testing.T) {
		// 		ctx, cancel := context.WithCancel(context.Background())
		// 		defer cancel()

		// 		lis, serverConnected, clientConnected := setupConnection(t)
		// 		defer lis.Close()

		// 		clientLocal := &clientLocal{}
		// 		_, serverDone := startServer(t, ctx, lis, serverConnected)
		// 		clientRegistry, clientDone := startClient(t, ctx, lis.Addr().String(), clientLocal, clientConnected)

		// 		// Wait for client to connect to server
		// 		serverConnected.Wait()
		// 		clientConnected.Wait()

		// 		// Track callback invocations
		// 		callbackCount := 0
		// 		expectedLength := 5

		// 		// Test client calling server with callback
		// 		err := clientRegistry.ForRemotes(func(remoteID string, remote clientRemote) error {
		// 			length, err := remote.Iterate(ctx, expectedLength, func(ctx context.Context, i int, b string) (string, error) {
		// 				callbackCount++

		// 				require.Equal(t, "This is from the server", b)

		// 				return "response from client", nil
		// 			})
		// 			require.NoError(t, err)

		// 			require.Equal(t, expectedLength, length)

		// 			return nil
		// 		})
		// 		require.NoError(t, err)

		// 		// Verify callback was called expected number of times
		// 		require.Equal(t, expectedLength, callbackCount)

		// 		cancel()
		// 		clientDone.Wait()
		// 		serverDone.Wait()
		// 	},
		// },
		// {
		// 	name: "nested callbacks between client and server",
		// 	run: func(t *testing.T) {
		// 		ctx, cancel := context.WithCancel(context.Background())
		// 		defer cancel()

		// 		lis, serverConnected, clientConnected := setupConnection(t)
		// 		defer lis.Close()

		// 		clientLocal := &clientLocal{}
		// 		serverRegistry, serverDone := startServer(t, ctx, lis, serverConnected)
		// 		clientRegistry, clientDone := startClient(t, ctx, lis.Addr().String(), clientLocal, clientConnected)

		// 		// Wait for client to connect to server
		// 		serverConnected.Wait()
		// 		clientConnected.Wait()

		// 		// Track nested callback invocations
		// 		outerCount := 0
		// 		innerCount := 0
		// 		expectedOuterLength := 3
		// 		expectedInnerLength := 2

		// 		// Test nested callbacks
		// 		err := clientRegistry.ForRemotes(func(remoteID string, remote clientRemote) error {
		// 			length, err := remote.Iterate(ctx, expectedOuterLength, func(ctx context.Context, i int, b string) (string, error) {
		// 				outerCount++
		// 				require.Equal(t, "This is from the server", b)

		// 				// Make nested callback from server back to client
		// 				var nestedErr error
		// 				err := serverRegistry.ForRemotes(func(remoteID string, remote serverRemote) error {
		// 					nestedLength, err := remote.Iterate(ctx, expectedInnerLength, func(ctx context.Context, i int, b string) (string, error) {
		// 						innerCount++

		// 						require.Equal(t, "This is from the client", b)

		// 						return "response from nested callback", nil
		// 					})
		// 					if err != nil {
		// 						nestedErr = err

		// 						return err
		// 					}

		// 					require.Equal(t, expectedInnerLength, nestedLength)

		// 					return nil
		// 				})
		// 				if nestedErr != nil {
		// 					return "", nestedErr
		// 				}

		// 				return "response from outer callback", err
		// 			})
		// 			require.NoError(t, err)

		// 			require.Equal(t, expectedOuterLength, length)

		// 			return nil
		// 		})
		// 		require.NoError(t, err)

		// 		// Verify callback counts
		// 		require.Equal(t, expectedOuterLength, outerCount)
		// 		require.Equal(t, expectedOuterLength*expectedInnerLength, innerCount)

		// 		cancel()
		// 		clientDone.Wait()
		// 		serverDone.Wait()
		// 	},
		// },
		// {
		// 	name: "bidirectional batch processing",
		// 	run: func(t *testing.T) {
		// 		ctx, cancel := context.WithCancel(context.Background())
		// 		defer cancel()

		// 		lis, serverConnected, clientConnected := setupConnection(t)
		// 		defer lis.Close()

		// 		clientLocal := &clientLocal{}
		// 		serverRegistry, serverDone := startServer(t, ctx, lis, serverConnected)
		// 		clientRegistry, clientDone := startClient(t, ctx, lis.Addr().String(), clientLocal, clientConnected)

		// 		// Wait for client to connect to server
		// 		serverConnected.Wait()
		// 		clientConnected.Wait()

		// 		// Test data
		// 		serverItems := []string{"server1", "server2", "server3"}
		// 		clientItems := []string{"client1", "client2", "client3"}

		// 		// Track callback invocations
		// 		serverCallbackItems := make([]string, 0)
		// 		clientCallbackItems := make([]string, 0)
		// 		var serverCallbackLock, clientCallbackLock sync.Mutex

		// 		// Test server calling client's batch processor
		// 		err := serverRegistry.ForRemotes(func(remoteID string, remote serverRemote) error {
		// 			processed, err := remote.ProcessBatch(ctx, serverItems, func(ctx context.Context, item string, i int) (bool, error) {
		// 				serverCallbackLock.Lock()
		// 				serverCallbackItems = append(serverCallbackItems, item)
		// 				serverCallbackLock.Unlock()

		// 				return true, nil
		// 			})
		// 			require.NoError(t, err)

		// 			require.Equal(t, len(serverItems), processed)

		// 			return nil
		// 		})
		// 		require.NoError(t, err)

		// 		// Test client calling server's batch processor
		// 		err = clientRegistry.ForRemotes(func(remoteID string, remote clientRemote) error {
		// 			processed, err := remote.ProcessBatch(ctx, clientItems, func(ctx context.Context, item string, i int) (bool, error) {
		// 				clientCallbackLock.Lock()
		// 				clientCallbackItems = append(clientCallbackItems, item)
		// 				clientCallbackLock.Unlock()

		// 				return true, nil
		// 			})
		// 			require.NoError(t, err)

		// 			require.Equal(t, len(clientItems), processed)

		// 			return nil
		// 		})
		// 		require.NoError(t, err)

		// 		// Verify callbacks received all items
		// 		require.Equal(t, serverItems, serverCallbackItems)
		// 		require.Equal(t, clientItems, clientCallbackItems)

		// 		cancel()
		// 		clientDone.Wait()
		// 		serverDone.Wait()
		// 	},
		// },
		// {
		// 	name: "batch processing with early termination",
		// 	run: func(t *testing.T) {
		// 		ctx, cancel := context.WithCancel(context.Background())
		// 		defer cancel()

		// 		lis, serverConnected, clientConnected := setupConnection(t)
		// 		defer lis.Close()

		// 		clientLocal := &clientLocal{}
		// 		serverRegistry, serverDone := startServer(t, ctx, lis, serverConnected)
		// 		clientRegistry, clientDone := startClient(t, ctx, lis.Addr().String(), clientLocal, clientConnected)

		// 		// Wait for client to connect to server
		// 		serverConnected.Wait()
		// 		clientConnected.Wait()

		// 		// Test data
		// 		serverItems := []string{"server1", "server2", "server3", "server4", "server5"}
		// 		clientItems := []string{"client1", "client2", "client3", "client4", "client5"}

		// 		// Track callback invocations
		// 		serverCallbackItems := make([]string, 0)
		// 		clientCallbackItems := make([]string, 0)
		// 		var serverCallbackLock, clientCallbackLock sync.Mutex

		// 		// Test server calling client's batch processor with early termination
		// 		err := serverRegistry.ForRemotes(func(remoteID string, remote serverRemote) error {
		// 			processed, err := remote.ProcessBatch(ctx, serverItems, func(ctx context.Context, item string, index int) (bool, error) {
		// 				serverCallbackLock.Lock()
		// 				serverCallbackItems = append(serverCallbackItems, item)
		// 				serverCallbackLock.Unlock()

		// 				return index < 2, nil
		// 			})
		// 			require.NoError(t, err)
		// 			require.Equal(t, 3, processed) // Should process first 3 items
		// 			return nil
		// 		})
		// 		require.NoError(t, err)

		// 		// Test client calling server's batch processor with early termination
		// 		err = clientRegistry.ForRemotes(func(remoteID string, remote clientRemote) error {
		// 			processed, err := remote.ProcessBatch(ctx, clientItems, func(ctx context.Context, item string, index int) (bool, error) {
		// 				clientCallbackLock.Lock()
		// 				clientCallbackItems = append(clientCallbackItems, item)
		// 				clientCallbackLock.Unlock()

		// 				return index < 3, nil
		// 			})
		// 			require.NoError(t, err)
		// 			require.Equal(t, 4, processed) // Should process first 4 items
		// 			return nil
		// 		})
		// 		require.NoError(t, err)

		// 		// Verify callbacks received correct items
		// 		require.Equal(t, []string{"server1", "server2", "server3"}, serverCallbackItems)
		// 		require.Equal(t, []string{"client1", "client2", "client3", "client4"}, clientCallbackItems)

		// 		cancel()
		// 		clientDone.Wait()
		// 		serverDone.Wait()
		// 	},
		// },
		// {
		// 	name: "batch processing with errors",
		// 	run: func(t *testing.T) {
		// 		ctx, cancel := context.WithCancel(context.Background())
		// 		defer cancel()

		// 		lis, serverConnected, clientConnected := setupConnection(t)
		// 		defer lis.Close()

		// 		clientLocal := &clientLocal{}
		// 		serverRegistry, serverDone := startServer(t, ctx, lis, serverConnected)
		// 		clientRegistry, clientDone := startClient(t, ctx, lis.Addr().String(), clientLocal, clientConnected)

		// 		// Wait for client to connect to server
		// 		serverConnected.Wait()
		// 		clientConnected.Wait()

		// 		// Test data
		// 		serverItems := []string{"server1", "server2", "server3", "server4", "server5"}
		// 		clientItems := []string{"client1", "client2", "client3", "client4", "client5"}

		// 		// Track callback invocations
		// 		serverCallbackItems := make([]string, 0)
		// 		clientCallbackItems := make([]string, 0)
		// 		var serverCallbackLock, clientCallbackLock sync.Mutex

		// 		// Test server calling client's batch processor with error
		// 		err := serverRegistry.ForRemotes(func(remoteID string, remote serverRemote) error {
		// 			processed, err := remote.ProcessBatch(ctx, serverItems, func(ctx context.Context, item string, index int) (bool, error) {
		// 				if index == 2 {
		// 					return false, errors.New("server-side error")
		// 				}

		// 				serverCallbackLock.Lock()
		// 				serverCallbackItems = append(serverCallbackItems, item)
		// 				serverCallbackLock.Unlock()

		// 				return true, nil
		// 			})
		// 			require.Error(t, err)
		// 			require.Equal(t, 2, processed) // Should process items until error
		// 			return nil
		// 		})
		// 		require.NoError(t, err)

		// 		// Test client calling server's batch processor with error
		// 		err = clientRegistry.ForRemotes(func(remoteID string, remote clientRemote) error {
		// 			processed, err := remote.ProcessBatch(ctx, clientItems, func(ctx context.Context, item string, index int) (bool, error) {
		// 				if index == 3 {
		// 					return false, errors.New("client-side error")
		// 				}

		// 				clientCallbackLock.Lock()
		// 				clientCallbackItems = append(clientCallbackItems, item)
		// 				clientCallbackLock.Unlock()

		// 				return true, nil
		// 			})
		// 			require.Error(t, err)
		// 			require.Equal(t, 3, processed) // Should process items until error
		// 			return nil
		// 		})
		// 		require.NoError(t, err)

		// 		// Verify callbacks received correct items
		// 		require.Equal(t, []string{"server1", "server2"}, serverCallbackItems)
		// 		require.Equal(t, []string{"client1", "client2", "client3"}, clientCallbackItems)

		// 		cancel()
		// 		clientDone.Wait()
		// 		serverDone.Wait()
		// 	},
		// },
		// {
		// 	name: "nested RPCs",
		// 	run: func(t *testing.T) {
		// 		ctx, cancel := context.WithCancel(context.Background())
		// 		defer cancel()

		// 		lis, serverConnected, clientConnected := setupConnection(t)
		// 		defer lis.Close()

		// 		_, serverDone := startServer(t, ctx, lis, serverConnected)
		// 		clientRegistry, clientDone := startClient(t, ctx, lis.Addr().String(), &clientLocal{}, clientConnected)

		// 		// Wait for client to connect to server
		// 		serverConnected.Wait()
		// 		clientConnected.Wait()

		// 		// Test nested service operations
		// 		err := clientRegistry.ForRemotes(func(remoteID string, remote clientRemote) error {
		// 			err := remote.Nested.SetValue(ctx, "test value")
		// 			require.NoError(t, err)

		// 			value, err := remote.Nested.GetValue(ctx)
		// 			require.NoError(t, err)
		// 			require.Equal(t, "test value", value)

		// 			err = remote.Nested.SetValue(ctx, "updated value")
		// 			require.NoError(t, err)

		// 			value, err = remote.Nested.GetValue(ctx)
		// 			require.NoError(t, err)
		// 			require.Equal(t, "updated value", value)

		// 			return nil
		// 		})
		// 		require.NoError(t, err)

		// 		cancel()
		// 		clientDone.Wait()
		// 		serverDone.Wait()
		// 	},
		// },
		// {
		// 	name: "bidirectional nested RPCs",
		// 	run: func(t *testing.T) {
		// 		ctx, cancel := context.WithCancel(context.Background())
		// 		defer cancel()

		// 		lis, serverConnected, clientConnected := setupConnection(t)
		// 		defer lis.Close()

		// 		// Initialize client with nested service
		// 		clientLocal := &clientLocal{
		// 			Nested: &nestedServiceLocal{
		// 				value: "initial client value",
		// 			},
		// 		}

		// 		serverRegistry, serverDone := startServer(t, ctx, lis, serverConnected)
		// 		clientRegistry, clientDone := startClient(t, ctx, lis.Addr().String(), clientLocal, clientConnected)

		// 		// Wait for client to connect to server
		// 		serverConnected.Wait()
		// 		clientConnected.Wait()

		// 		// Test client calling server's nested service
		// 		err := clientRegistry.ForRemotes(func(remoteID string, remote clientRemote) error {
		// 			// Set and verify server value
		// 			err := remote.Nested.SetValue(ctx, "test server value")
		// 			require.NoError(t, err)

		// 			value, err := remote.Nested.GetValue(ctx)
		// 			require.NoError(t, err)
		// 			require.Equal(t, "test server value", value)

		// 			return nil
		// 		})
		// 		require.NoError(t, err)

		// 		// Test server calling client's nested service
		// 		err = serverRegistry.ForRemotes(func(remoteID string, remote serverRemote) error {
		// 			// Set and verify client value
		// 			err := remote.Nested.SetValue(ctx, "test client value")
		// 			require.NoError(t, err)

		// 			value, err := remote.Nested.GetValue(ctx)
		// 			require.NoError(t, err)
		// 			require.Equal(t, "test client value", value)

		// 			// Test concurrent access to both nested services
		// 			var wg sync.WaitGroup
		// 			wg.Add(2)

		// 			go func() {
		// 				defer wg.Done()

		// 				value, err := remote.Nested.GetValue(ctx)
		// 				require.NoError(t, err)

		// 				require.Equal(t, "test client value", value)
		// 			}()

		// 			go func() {
		// 				defer wg.Done()

		// 				err := clientRegistry.ForRemotes(func(remoteID string, remote clientRemote) error {
		// 					value, err := remote.Nested.GetValue(ctx)
		// 					require.NoError(t, err)

		// 					require.Equal(t, "test server value", value)

		// 					return nil
		// 				})
		// 				require.NoError(t, err)
		// 			}()

		// 			wg.Wait()

		// 			return nil
		// 		})
		// 		require.NoError(t, err)

		// 		// Test nested service state persistence
		// 		err = clientRegistry.ForRemotes(func(remoteID string, remote clientRemote) error {
		// 			value, err := remote.Nested.GetValue(ctx)
		// 			require.NoError(t, err)

		// 			require.Equal(t, "test server value", value)

		// 			return nil
		// 		})
		// 		require.NoError(t, err)

		// 		err = serverRegistry.ForRemotes(func(remoteID string, remote serverRemote) error {
		// 			value, err := remote.Nested.GetValue(ctx)
		// 			require.NoError(t, err)

		// 			require.Equal(t, "test client value", value)

		// 			return nil
		// 		})
		// 		require.NoError(t, err)

		// 		cancel()
		// 		clientDone.Wait()
		// 		serverDone.Wait()
		// 	},
		// },
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tt.run(t)
		})
	}
}
