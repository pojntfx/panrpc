package rpc

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/teivah/broadcast"
)

var (
	errorType   = reflect.TypeOf((*error)(nil)).Elem()
	contextType = reflect.TypeOf((*context.Context)(nil)).Elem()

	ErrInvalidReturn = errors.New("invalid return, can only return an error or a value and an error")
	ErrInvalidArgs   = errors.New("invalid arguments, first argument needs to be a context.Context")

	ErrInvalidRequest        = errors.New("invalid request")
	ErrCannotCallNonFunction = errors.New("can not call non function")
	ErrCallTimedOut          = errors.New("call timed out")
)

type key int

const (
	remoteIDContextKey key = iota

	DefaultResponseBufferLen = 1024
)

type response struct {
	id      string
	value   json.RawMessage
	err     error
	timeout bool
}

func GetRemoteID(ctx context.Context) string {
	return ctx.Value(remoteIDContextKey).(string)
}

type Options struct {
	ResponseBufferLen  int
	OnClientConnect    func(remoteID string)
	OnClientDisconnect func(remoteID string)
}

type Registry[R any] struct {
	local  any
	remote R

	remotes     map[string]R
	remotesLock *sync.Mutex

	timeout time.Duration
	ctx     context.Context

	options *Options
}

func NewRegistry[R any](
	local any,
	remote R,

	timeout time.Duration,
	ctx context.Context,

	options *Options,
) *Registry[R] {
	if options == nil {
		options = &Options{
			ResponseBufferLen: DefaultResponseBufferLen,
		}
	}

	return &Registry[R]{local, remote, map[string]R{}, &sync.Mutex{}, timeout, ctx, options}
}

func (r Registry[R]) Link(conn io.ReadWriteCloser) error {
	responseResolver := broadcast.NewRelay[response]()

	remote := reflect.New(reflect.ValueOf(r.remote).Type()).Elem()

	errs := make(chan error)

	go func() {
		for i := 0; i < remote.NumField(); i++ {
			functionField := remote.Type().Field(i)
			functionType := functionField.Type

			if functionType.Kind() != reflect.Func {
				continue
			}

			if functionType.NumOut() <= 0 || functionType.NumOut() > 2 {
				errs <- ErrInvalidReturn

				break
			}

			if !functionType.Out(functionType.NumOut() - 1).Implements(errorType) {
				errs <- ErrInvalidReturn

				break
			}

			if functionType.NumIn() < 1 {
				errs <- ErrInvalidArgs

				break
			}

			if !functionType.In(0).Implements(contextType) {
				errs <- ErrInvalidArgs

				break
			}

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
					errs <- err

					return
				}

				l := responseResolver.Listener(r.options.ResponseBufferLen)
				defer l.Close()

				t := time.NewTimer(r.timeout)
				defer t.Stop()

				res := make(chan response)
				go func() {
					for {
						select {
						case <-t.C:
							t.Stop()

							res <- response{"", nil, ErrCallTimedOut, true}

							return
						case msg, ok := <-l.Ch():
							if !ok {
								return
							}

							if msg.id == callID {
								res <- msg

								return
							}
						}
					}
				}()

				if _, err := conn.Write(b); err != nil {
					errs <- err

					return
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
								errs <- err

								return
							}
						}

						returnValues = append(returnValues, returnValue.Elem())
					} else if functionType.NumOut() == 2 {
						valueReturnValue := reflect.New(functionType.Out(0))
						errReturnValue := reflect.New(functionType.Out(1))

						if !rawReturnValue.timeout {
							if err := json.Unmarshal(rawReturnValue.value, valueReturnValue.Interface()); err != nil {
								errs <- err

								return
							}
						}

						if rawReturnValue.err != nil {
							errReturnValue.Elem().Set(reflect.ValueOf(rawReturnValue.err))
						}

						returnValues = append(returnValues, valueReturnValue.Elem(), errReturnValue.Elem())
					}
				case <-r.ctx.Done():
					errs <- r.ctx.Err()

					return
				}

				return returnValues
			})

			remote.FieldByName(functionField.Name).Set(fn)
		}

		remoteID := uuid.NewString()

		r.remotesLock.Lock()
		r.remotes[remoteID] = remote.Interface().(R)

		if r.options.OnClientConnect != nil {
			r.options.OnClientConnect(remoteID)
		}
		r.remotesLock.Unlock()

		defer func() {
			r.remotesLock.Lock()
			delete(r.remotes, remoteID)

			if r.options.OnClientDisconnect != nil {
				r.options.OnClientDisconnect(remoteID)
			}
			r.remotesLock.Unlock()
		}()

		d := json.NewDecoder(conn)

		for {
			var res []json.RawMessage
			if err := d.Decode(&res); err != nil {
				errs <- err

				return
			}

			if len(res) != 4 {
				errs <- ErrInvalidRequest

				return
			}

			var isCall bool
			if err := json.Unmarshal(res[0], &isCall); err != nil {
				errs <- err

				return
			}

			var callID string
			if err := json.Unmarshal(res[1], &callID); err != nil {
				errs <- err

				return
			}

			if isCall {
				go func() {
					var functionName string
					if err := json.Unmarshal(res[2], &functionName); err != nil {
						errs <- err

						return
					}

					var functionArgs []json.RawMessage
					if err := json.Unmarshal(res[3], &functionArgs); err != nil {
						errs <- err

						return
					}

					function := reflect.
						ValueOf(r.local).
						MethodByName(functionName)

					if function.Kind() != reflect.Func {
						errs <- ErrCannotCallNonFunction

						return
					}

					if function.Type().NumIn() != len(functionArgs)+1 {
						errs <- ErrInvalidArgs

						return
					}

					args := []reflect.Value{}
					for i := 0; i < function.Type().NumIn(); i++ {
						if i == 0 {
							// Add the context to the function arguments
							args = append(args, reflect.ValueOf(context.WithValue(r.ctx, remoteIDContextKey, remoteID)))

							continue
						}

						arg := reflect.New(function.Type().In(i))
						if err := json.Unmarshal(functionArgs[i-1], arg.Interface()); err != nil {
							errs <- err

							return
						}

						args = append(args, arg.Elem())
					}

					go func() {
						res := function.Call(args)

						switch len(res) {
						case 0:
							b, err := json.Marshal([]interface{}{false, callID, nil, ""})
							if err != nil {
								errs <- err

								return
							}

							if _, err := conn.Write(b); err != nil {
								errs <- err

								return
							}
						case 1:
							if res[0].Type().Implements(errorType) && !res[0].IsNil() {
								b, err := json.Marshal([]interface{}{false, callID, nil, res[0].Interface().(error).Error()})
								if err != nil {
									errs <- err

									return
								}

								if _, err := conn.Write(b); err != nil {
									errs <- err

									return
								}
							} else {
								v, err := json.Marshal(res[0].Interface())
								if err != nil {
									errs <- err

									return
								}

								b, err := json.Marshal([]interface{}{false, callID, json.RawMessage(string(v)), ""})
								if err != nil {
									errs <- err

									return
								}

								if _, err := conn.Write(b); err != nil {
									errs <- err

									return
								}
							}
						case 2:
							v, err := json.Marshal(res[0].Interface())
							if err != nil {
								errs <- err

								return
							}

							if res[1].Interface() == nil {
								b, err := json.Marshal([]interface{}{false, callID, json.RawMessage(string(v)), ""})
								if err != nil {
									errs <- err

									return
								}

								if _, err := conn.Write(b); err != nil {
									errs <- err

									return
								}
							} else {
								b, err := json.Marshal([]interface{}{false, callID, json.RawMessage(string(v)), res[1].Interface().(error).Error()})
								if err != nil {
									errs <- err

									return
								}

								if _, err := conn.Write(b); err != nil {
									errs <- err

									return
								}
							}
						}
					}()
				}()

				continue
			}

			var errMsg string
			if err := json.Unmarshal(res[3], &errMsg); err != nil {
				errs <- err

				return
			}

			var err error
			if strings.TrimSpace(errMsg) != "" {
				err = errors.New(errMsg)
			}

			responseResolver.Broadcast(response{callID, res[2], err, false})
		}
	}()

	return <-errs
}

func (r Registry[R]) Peers() map[string]R {
	return r.remotes
}
