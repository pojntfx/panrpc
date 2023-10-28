package rpc

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pojntfx/dudirekta/pkg/utils"
)

var (
	errorType   = reflect.TypeOf((*error)(nil)).Elem()
	contextType = reflect.TypeOf((*context.Context)(nil)).Elem()

	ErrInvalidReturn = errors.New("invalid return, can only return an error or a value and an error")
	ErrInvalidArgs   = errors.New("invalid arguments, first argument needs to be a context.Context")

	ErrCannotCallNonFunction = errors.New("can not call non function")
	ErrCallTimedOut          = errors.New("call timed out")
)

type key int

const (
	RemoteIDContextKey key = iota

	DefaultResponseBufferLen = 1024
)

type Message[T any] struct {
	Request  *T `json:"request"`
	Response *T `json:"response"`
}

type Request[T any] struct {
	Call     string `json:"call"`
	Function string `json:"function"`
	Args     []T    `json:"args"`
}

type Response[T any] struct {
	Call  string `json:"call"`
	Value T      `json:"value"`
	Err   string `json:"err"`
}

type callResponse[T any] struct {
	value   T
	err     error
	timeout bool
}

type wrappedChild struct {
	wrappee any
	wrapper *closureManager
}

func GetRemoteID(ctx context.Context) string {
	return ctx.Value(RemoteIDContextKey).(string)
}

type Options struct {
	OnClientConnect    func(remoteID string)
	OnClientDisconnect func(remoteID string)
}

type Registry[R, T any] struct {
	local  wrappedChild
	remote R

	remotes     map[string]R
	remotesLock *sync.Mutex

	timeout time.Duration
	ctx     context.Context

	options *Options
}

func NewRegistry[R, T any](
	local any,

	timeout time.Duration,
	ctx context.Context,

	options *Options,
) *Registry[R, T] {
	if options == nil {
		options = &Options{}
	}

	return &Registry[R, T]{wrappedChild{
		local,
		&closureManager{
			closuresLock: sync.Mutex{},
			closures:     map[string]func(args ...interface{}) (interface{}, error){},
		},
	}, *new(R), map[string]R{}, &sync.Mutex{}, timeout, ctx, options}
}

func (r Registry[R, T]) makeRPC(
	name string,
	functionType reflect.Type,
	setErr func(err error),
	responseResolver *utils.Broadcaster[callResponse[T]],

	writeRequest func(b T) error,

	marshalNested func(v any) (T, error),
	unmarshalNested func(data T, v any) error,
) reflect.Value {
	return reflect.MakeFunc(functionType, func(args []reflect.Value) (results []reflect.Value) {
		defer func() {
			var err error
			if e := recover(); e != nil {
				var ok bool
				err, ok = e.(error)
				if !ok {
					err = utils.ErrPanickedWithNonErrorValue
				}

				setErr(err)
			}

			// If we tried to return with an invalid results count, set them so that the call doesn't panic
			if len(results) != functionType.NumOut() {
				errReturnValue := reflect.ValueOf(err)

				if functionType.NumOut() == 1 {
					results = []reflect.Value{errReturnValue}
				} else if functionType.NumOut() == 2 {
					valueReturnValue := reflect.Zero(functionType.Out(0))

					results = []reflect.Value{valueReturnValue, errReturnValue}
				}
			}
		}()

		callID := uuid.NewString()

		cmd := Request[T]{
			Call:     callID,
			Function: name,
			Args:     []T{},
		}

		for i, arg := range args {
			if i == 0 {
				// Don't sent the context over the wire

				continue
			}

			if arg.Kind() == reflect.Func {
				closureID, freeClosure, err := registerClosure(r.local.wrapper, arg.Interface())
				if err != nil {
					panic(err)
				}
				defer freeClosure()

				b, err := marshalNested(closureID)
				if err != nil {
					panic(err)
				}
				cmd.Args = append(cmd.Args, b)
			} else {
				b, err := marshalNested(arg.Interface())
				if err != nil {
					panic(err)
				}
				cmd.Args = append(cmd.Args, b)
			}
		}

		b, err := marshalNested(cmd)
		if err != nil {
			panic(err)
		}

		res := make(chan callResponse[T])
		go func() {
			defer responseResolver.Close(callID)

			r, ok := responseResolver.Receive(callID, r.timeout)
			if !ok {
				r = &callResponse[T]{*new(T), ErrCallTimedOut, true}
			}

			res <- *r
		}()

		if err := writeRequest(b); err != nil {
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
					if err := unmarshalNested(rawReturnValue.value, returnValue.Interface()); err != nil {
						panic(err)
					}
				}

				returnValues = append(returnValues, returnValue.Elem())
			} else if functionType.NumOut() == 2 {
				valueReturnValue := reflect.New(functionType.Out(0))
				errReturnValue := reflect.New(functionType.Out(1))

				if !rawReturnValue.timeout {
					if err := unmarshalNested(rawReturnValue.value, valueReturnValue.Interface()); err != nil {
						panic(err)
					}
				}

				if rawReturnValue.err != nil {
					errReturnValue.Elem().Set(reflect.ValueOf(rawReturnValue.err))
				}

				returnValues = append(returnValues, valueReturnValue.Elem(), errReturnValue.Elem())
			}
		case <-r.ctx.Done():
			panic(r.ctx.Err())
		}

		return returnValues
	})
}

func (r Registry[R, T]) LinkStream(
	encode func(v any) error,
	decode func(v any) error,

	marshal func(v any) ([]byte, error),
	unmarshal func(data []byte, v any) error,

	marshalNested func(v any) (T, error),
	unmarshalNested func(data T, v any) error,
) error {
	var (
		decodeDone = make(chan struct{})
		decodeErr  error

		requests  = make(chan T)
		responses = make(chan T)
	)
	go func() {
		for {
			var msg Message[T]
			if err := decode(&msg); err != nil {
				decodeErr = err

				close(decodeDone)

				break
			}

			if msg.Request != nil {
				requests <- *msg.Request
			}

			if msg.Response != nil {
				responses <- *msg.Response
			}
		}
	}()

	return r.LinkMessage(
		func(b T) error {
			return encode(Message[T]{
				Request: &b,
			})
		},
		func(b T) error {
			return encode(Message[T]{
				Response: &b,
			})
		},

		func() (T, error) {
			select {
			case <-decodeDone:
				return *new(T), decodeErr
			case request := <-requests:
				return request, nil
			}
		},
		func() (T, error) {
			select {
			case <-decodeDone:
				return *new(T), decodeErr
			case response := <-responses:
				return response, nil
			}
		},

		marshalNested,
		unmarshalNested,
	)
}

func (r Registry[R, T]) LinkMessage(
	writeRequest,
	writeResponse func(b T) error,

	readRequest,
	readResponse func() (T, error),

	marshalNested func(v any) (T, error),
	unmarshalNested func(data T, v any) error,
) error {
	responseResolver := utils.NewBroadcaster[callResponse[T]]()

	remote := reflect.New(reflect.ValueOf(r.remote).Type()).Elem()

	var fatalErr error
	fatalErrLock := sync.NewCond(&sync.Mutex{})

	setErr := func(err error) {
		fatalErrLock.L.Lock()
		fatalErr = err
		fatalErrLock.Broadcast()
		fatalErrLock.L.Unlock()
	}

	go func() {
		for i := 0; i < remote.NumField(); i++ {
			functionField := remote.Type().Field(i)
			functionType := functionField.Type

			if functionType.Kind() != reflect.Func {
				continue
			}

			if functionType.NumOut() <= 0 || functionType.NumOut() > 2 {
				setErr(ErrInvalidReturn)

				break
			}

			if !functionType.Out(functionType.NumOut() - 1).Implements(errorType) {
				setErr(ErrInvalidReturn)

				break
			}

			if functionType.NumIn() < 1 {
				setErr(ErrInvalidArgs)

				break
			}

			if !functionType.In(0).Implements(contextType) {
				setErr(ErrInvalidArgs)

				break
			}

			remote.
				FieldByName(functionField.Name).
				Set(r.makeRPC(
					functionField.Name,
					functionType,
					setErr,
					responseResolver,

					writeRequest,

					marshalNested,
					unmarshalNested,
				))
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

		var wg sync.WaitGroup

		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				b, err := readRequest()
				if err != nil {
					setErr(err)

					return
				}

				var req Request[T]
				if err := unmarshalNested(b, &req); err != nil {
					setErr(err)

					return
				}

				go func() {
					function := reflect.
						ValueOf(r.local.wrappee).
						MethodByName(req.Function)

					if function.Kind() != reflect.Func {
						function = reflect.
							ValueOf(r.local.wrapper).
							MethodByName(req.Function)

						if function.Kind() != reflect.Func {
							setErr(ErrCannotCallNonFunction)

							return
						}
					}

					if function.Type().NumIn() != len(req.Args)+1 {
						setErr(ErrInvalidArgs)

						return
					}

					args := []reflect.Value{}
					for i := 0; i < function.Type().NumIn(); i++ {
						if i == 0 {
							// Add the context to the function arguments
							args = append(args, reflect.ValueOf(context.WithValue(r.ctx, RemoteIDContextKey, remoteID)))

							continue
						}

						functionType := function.Type().In(i)
						if functionType.Kind() == reflect.Func {
							arg := reflect.MakeFunc(functionType, func(args []reflect.Value) (results []reflect.Value) {
								defer func() {
									var err error
									if e := recover(); e != nil {
										var ok bool
										err, ok = e.(error)
										if !ok {
											err = utils.ErrPanickedWithNonErrorValue
										}

										setErr(err)
									}

									// If we tried to return with an invalid results count, set them so that the call doesn't panic
									if len(results) != functionType.NumOut() {
										errReturnValue := reflect.ValueOf(err)

										if functionType.NumOut() == 1 {
											results = []reflect.Value{errReturnValue}
										} else if functionType.NumOut() == 2 {
											valueReturnValue := reflect.Zero(functionType.Out(0))

											results = []reflect.Value{valueReturnValue, errReturnValue}
										}
									}
								}()

								closureID := ""
								if err := unmarshalNested(req.Args[i-2], &closureID); err != nil {
									panic(err)
								}

								rpc := r.makeRPC(
									"CallClosure",
									reflect.TypeOf(callClosureType(nil)),
									setErr,
									responseResolver,

									writeRequest,

									marshalNested,
									unmarshalNested,
								)

								rpcArgs := []interface{}{}
								for i := 0; i < len(args); i++ {
									rpcArgs = append(rpcArgs, args[i].Interface())
								}

								rcpRv, err := utils.Call(rpc, []reflect.Value{reflect.ValueOf(r.ctx), reflect.ValueOf(closureID), reflect.ValueOf(rpcArgs)})
								if err != nil {
									panic(err)
								}

								rv := []reflect.Value{}
								if functionType.NumOut() == 1 {
									returnValue := reflect.New(functionType.Out(0))

									returnValue.Elem().Set(rcpRv[1]) // Error return value is at index 1

									rv = append(rv, returnValue.Elem())
								} else if functionType.NumOut() == 2 {
									valueReturnValue := reflect.New(functionType.Out(0))
									errReturnValue := reflect.New(functionType.Out(1))

									if el := rcpRv[0].Elem(); el.IsValid() {
										valueReturnValue.Elem().Set(el.Convert(valueReturnValue.Type().Elem()))
									}
									errReturnValue.Elem().Set(rcpRv[1])

									rv = append(rv, valueReturnValue.Elem(), errReturnValue.Elem())
								}

								return rv
							})

							args = append(args, arg)
						} else {
							arg := reflect.New(functionType)

							if err := unmarshalNested(req.Args[i-1], arg.Interface()); err != nil {
								setErr(err)

								return
							}

							args = append(args, arg.Elem())
						}
					}

					go func() {
						res, err := utils.Call(function, args)
						if err != nil {
							setErr(err)

							return
						}

						switch len(res) {
						case 0:
							v, err := marshalNested(nil)
							if err != nil {
								setErr(err)

								return
							}

							b, err := marshalNested(Response[T]{
								Call:  req.Call,
								Value: v,
								Err:   "",
							})
							if err != nil {
								setErr(err)

								return
							}

							if err := writeResponse(b); err != nil {
								setErr(err)

								return
							}
						case 1:
							if res[0].Type().Implements(errorType) && !res[0].IsNil() {
								v, err := marshalNested(nil)
								if err != nil {
									setErr(err)

									return
								}

								b, err := marshalNested(Response[T]{
									Call:  req.Call,
									Value: v,
									Err:   res[0].Interface().(error).Error(),
								})
								if err != nil {
									setErr(err)

									return
								}

								if err := writeResponse(b); err != nil {
									setErr(err)

									return
								}
							} else {
								v, err := marshalNested(res[0].Interface())
								if err != nil {
									setErr(err)

									return
								}

								b, err := marshalNested(Response[T]{
									Call:  req.Call,
									Value: v,
									Err:   "",
								})
								if err != nil {
									setErr(err)

									return
								}

								if err := writeResponse(b); err != nil {
									setErr(err)

									return
								}
							}
						case 2:
							v, err := marshalNested(res[0].Interface())
							if err != nil {
								setErr(err)

								return
							}

							if res[1].Interface() == nil {
								b, err := marshalNested(Response[T]{
									Call:  req.Call,
									Value: v,
									Err:   "",
								})
								if err != nil {
									setErr(err)

									return
								}

								if err := writeResponse(b); err != nil {
									setErr(err)

									return
								}
							} else {
								b, err := marshalNested(Response[T]{
									Call:  req.Call,
									Value: v,
									Err:   res[1].Interface().(error).Error(),
								})
								if err != nil {
									setErr(err)

									return
								}

								if err := writeResponse(b); err != nil {
									setErr(err)

									return
								}
							}
						}
					}()
				}()
			}
		}()

		wg.Add(1)
		go func() {
			defer wg.Done()

			for {
				b, err := readResponse()
				if err != nil {
					setErr(err)

					return
				}

				var res Response[T]
				if err := unmarshalNested(b, &res); err != nil {
					setErr(err)

					return
				}

				if strings.TrimSpace(res.Err) != "" {
					err = errors.New(res.Err)
				}

				go func() {
					if ok := responseResolver.Publish(res.Call, callResponse[T]{res.Value, err, false}, r.timeout); !ok {
						responseResolver.Close(res.Call)
					}
				}()
			}
		}()

		wg.Wait()
	}()

	fatalErrLock.L.Lock()
	if fatalErr == nil {
		fatalErrLock.Wait()
	}
	fatalErrLock.L.Unlock()

	return fatalErr
}

func (r Registry[R, T]) ForRemotes(
	cb func(remoteID string, remote R) error,
) error {
	r.remotesLock.Lock()
	defer r.remotesLock.Unlock()

	for remoteID, remote := range r.remotes {
		if err := cb(remoteID, remote); err != nil {
			return err
		}
	}

	return nil
}
