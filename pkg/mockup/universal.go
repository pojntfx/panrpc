package mockup

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"net"
	"reflect"
	"strings"
	"sync"

	"github.com/google/uuid"
	"github.com/teivah/broadcast"
)

var (
	errorType = reflect.TypeOf((*error)(nil)).Elem()
)

type response struct {
	id    string
	value json.RawMessage
	err   error
}

type Registry[R any] struct {
	local  any
	remote R

	remotes     map[string]R
	remotesLock *sync.Mutex

	ctx context.Context
}

func NewRegistry[R any](
	local any,
	remote R,
	ctx context.Context,
) *Registry[R] {
	return &Registry[R]{local, remote, map[string]R{}, &sync.Mutex{}, ctx}
}

func (r Registry[R]) Listen(lis net.Listener) error {
	clients := 0

	for {
		func() {
			conn, err := lis.Accept()
			if err != nil {
				log.Println("could not accept connection, continuing:", err)

				return
			}

			go func() {
				clients++

				log.Printf("%v clients connected", clients)

				defer func() {
					clients--

					if err := recover(); err != nil {
						log.Printf("Client disconnected with error: %v", err)
					}

					log.Printf("%v clients connected", clients)
				}()

				if err := r.Connect(conn); err != nil {
					panic(err)
				}
			}()
		}()
	}
}

func (r Registry[R]) Connect(conn net.Conn) error {
	responseResolver := broadcast.NewRelay[response]()

	remote := reflect.New(reflect.ValueOf(r.remote).Type()).Elem()

	for i := 0; i < remote.NumField(); i++ {
		functionField := remote.Type().Field(i)
		functionType := functionField.Type

		fn := reflect.MakeFunc(functionType, func(args []reflect.Value) (results []reflect.Value) {
			callID := uuid.NewString()

			cmd := []any{true, callID, functionField.Name}

			cmdArgs := []any{}
			for i, arg := range args {
				if i == 0 {
					// Don't sent the context over the wire

					continue
				}

				cmdArgs = append(cmdArgs, arg.Interface())
			}
			cmd = append(cmd, cmdArgs)

			b, err := json.Marshal(cmd)
			if err != nil {
				panic(err)
			}

			l := responseResolver.Listener(0)
			defer l.Close()

			res := make(chan response)
			go func() {
				for msg := range l.Ch() {
					if msg.id == callID {
						res <- msg

						return
					}
				}
			}()

			if _, err := conn.Write(b); err != nil {
				panic(err)
			}

			returnValues := []reflect.Value{}
			select {
			case rawReturnValue := <-res:
				if functionType.NumOut() == 1 {
					returnValue := reflect.New(functionType.Out(0))

					if rawReturnValue.err != nil {
						returnValue.Elem().Set(reflect.ValueOf(rawReturnValue.err))
					} else if !functionType.Out(0).Implements(errorType) {
						if err := json.Unmarshal(rawReturnValue.value, returnValue.Interface()); err != nil {
							panic(err)
						}
					}

					returnValues = append(returnValues, returnValue.Elem())
				} else if functionType.NumOut() == 2 {
					valueReturnValue := reflect.New(functionType.Out(0))
					errReturnValue := reflect.New(functionType.Out(1))

					if err := json.Unmarshal(rawReturnValue.value, valueReturnValue.Interface()); err != nil {
						panic(err)
					}

					if rawReturnValue.err != nil {
						errReturnValue.Elem().Set(reflect.ValueOf(rawReturnValue.err))
					}

					returnValues = append(returnValues, valueReturnValue.Elem(), errReturnValue.Elem())
				}
			case err := <-r.ctx.Done():
				panic(err)
			}

			return returnValues
		})

		remote.FieldByName(functionField.Name).Set(fn)
	}

	remoteID := uuid.NewString()

	r.remotesLock.Lock()
	r.remotes[remoteID] = remote.Interface().(R)
	r.remotesLock.Unlock()

	d := json.NewDecoder(conn)

	for {
		var res []json.RawMessage
		if err := d.Decode(&res); err != nil {
			panic(err)
		}

		var isCall bool
		if err := json.Unmarshal(res[0], &isCall); err != nil {
			panic(err)
		}

		var callID string
		if err := json.Unmarshal(res[1], &callID); err != nil {
			panic(err)
		}

		if isCall {
			go func() {
				var functionName string
				if err := json.Unmarshal(res[2], &functionName); err != nil {
					panic(err)
				}

				var functionArgs []json.RawMessage
				if err := json.Unmarshal(res[3], &functionArgs); err != nil {
					panic(err)
				}

				function := reflect.
					ValueOf(r.local).
					Elem().
					MethodByName(functionName)

				args := []reflect.Value{}
				for i := 0; i < function.Type().NumIn(); i++ {
					if i == 0 {
						// Add the context to the function arguments
						args = append(args, reflect.ValueOf(r.ctx))

						continue
					}

					arg := reflect.New(function.Type().In(i))
					if err := json.Unmarshal(functionArgs[i-1], arg.Interface()); err != nil {
						panic(err)
					}

					args = append(args, arg.Elem())
				}

				go func() {
					res := function.Call(args)
					switch len(res) {
					case 0:
						b, err := json.Marshal([]interface{}{false, callID, nil, ""})
						if err != nil {
							panic(err)
						}

						if _, err := conn.Write(b); err != nil {
							panic(err)
						}
					case 1:
						if res[0].Type().Implements(errorType) && !res[0].IsNil() {
							b, err := json.Marshal([]interface{}{false, callID, nil, res[0].Interface().(error).Error()})
							if err != nil {
								panic(err)
							}

							if _, err := conn.Write(b); err != nil {
								panic(err)
							}
						} else {
							v, err := json.Marshal(res[0].Interface())
							if err != nil {
								panic(err)
							}

							b, err := json.Marshal([]interface{}{false, callID, json.RawMessage(string(v)), ""})
							if err != nil {
								panic(err)
							}

							if _, err := conn.Write(b); err != nil {
								panic(err)
							}
						}
					case 2:
						v, err := json.Marshal(res[0].Interface())
						if err != nil {
							panic(err)
						}

						if res[1].Interface() == nil {
							b, err := json.Marshal([]interface{}{false, callID, json.RawMessage(string(v)), ""})
							if err != nil {
								panic(err)
							}

							if _, err := conn.Write(b); err != nil {
								panic(err)
							}
						} else {
							b, err := json.Marshal([]interface{}{false, callID, json.RawMessage(string(v)), res[1].Interface().(error).Error()})
							if err != nil {
								panic(err)
							}

							if _, err := conn.Write(b); err != nil {
								panic(err)
							}
						}
					}
				}()
			}()

			continue
		}

		var errMsg string
		if err := json.Unmarshal(res[3], &errMsg); err != nil {
			panic(err)
		}

		var err error
		if strings.TrimSpace(errMsg) != "" {
			err = errors.New(errMsg)
		}

		responseResolver.Broadcast(response{callID, res[2], err})
	}
}

func (r Registry[R]) Peers() map[string]R {
	return r.remotes
}
