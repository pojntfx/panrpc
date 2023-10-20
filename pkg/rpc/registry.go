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

type Message struct {
	Request  *[]byte `json:"request"`
	Response *[]byte `json:"response"`
}

type Request struct {
	Call     string   `json:"call"`
	Function string   `json:"function"`
	Args     [][]byte `json:"args"`
}

type Response struct {
	Call  string `json:"call"`
	Value []byte `json:"value"`
	Err   string `json:"err"`
}

type callResponse struct {
	value   []byte
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

type Registry[R any] struct {
	local  wrappedChild
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
		options = &Options{}
	}

	return &Registry[R]{wrappedChild{
		local,
		&closureManager{
			closuresLock: sync.Mutex{},
			closures:     map[string]func(args ...interface{}) (interface{}, error){},
		},
	}, remote, map[string]R{}, &sync.Mutex{}, timeout, ctx, options}
}

func (r Registry[R]) makeRPC(
	name string,
	functionType reflect.Type,
	setErr func(err error),
	responseResolver *utils.Broadcaster[callResponse],

	writeRequest func(b []byte) error,

	marshal func(v any) ([]byte, error),
	unmarshal func(data []byte, v any) error,
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

		cmd := Request{
			Call:     callID,
			Function: name,
			Args:     [][]byte{},
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

				b, err := marshal(closureID)
				if err != nil {
					panic(err)
				}
				cmd.Args = append(cmd.Args, b)
			} else {
				b, err := marshal(arg.Interface())
				if err != nil {
					panic(err)
				}
				cmd.Args = append(cmd.Args, b)
			}
		}

		b, err := marshal(cmd)
		if err != nil {
			panic(err)
		}

		res := make(chan callResponse)
		go func() {
			defer responseResolver.Close(callID)

			r, ok := responseResolver.Receive(callID, r.timeout)
			if !ok {
				r = &callResponse{nil, ErrCallTimedOut, true}
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
					if err := unmarshal(rawReturnValue.value, returnValue.Interface()); err != nil {
						panic(err)
					}
				}

				returnValues = append(returnValues, returnValue.Elem())
			} else if functionType.NumOut() == 2 {
				valueReturnValue := reflect.New(functionType.Out(0))
				errReturnValue := reflect.New(functionType.Out(1))

				if !rawReturnValue.timeout {
					if err := unmarshal(rawReturnValue.value, valueReturnValue.Interface()); err != nil {
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

func (r Registry[R]) LinkStream(
	encode func(v any) error,
	decode func(v any) error,

	marshal func(v any) ([]byte, error),
	unmarshal func(data []byte, v any) error,
) error {
	var (
		decodeDone = make(chan struct{})
		decodeErr  error

		requests  = make(chan []byte)
		responses = make(chan []byte)
	)
	go func() {
		for {
			var msg Message
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
		func(b []byte) error {
			return encode(Message{
				Request: &b,
			})
		},
		func(b []byte) error {
			return encode(Message{
				Response: &b,
			})
		},

		func() ([]byte, error) {
			select {
			case <-decodeDone:
				return []byte{}, decodeErr
			case request := <-requests:
				return request, nil
			}
		},
		func() ([]byte, error) {
			select {
			case <-decodeDone:
				return []byte{}, decodeErr
			case response := <-responses:
				return response, nil
			}
		},

		marshal,
		unmarshal,
	)
}

func (r Registry[R]) LinkMessage(
	writeRequest,
	writeResponse func(b []byte) error,

	readRequest,
	readResponse func() ([]byte, error),

	marshal func(v any) ([]byte, error),
	unmarshal func(data []byte, v any) error,
) error {
	responseResolver := utils.NewBroadcaster[callResponse]()

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

					marshal,
					unmarshal,
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

				var req Request
				if err := unmarshal(b, &req); err != nil {
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
								if err := unmarshal(req.Args[i-2], &closureID); err != nil {
									panic(err)
								}

								rpc := r.makeRPC(
									"CallClosure",
									reflect.TypeOf(callClosureType(nil)),
									setErr,
									responseResolver,

									writeRequest,

									marshal,
									unmarshal,
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

							if err := unmarshal(req.Args[i-1], arg.Interface()); err != nil {
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
							b, err := marshal(Response{
								Call:  req.Call,
								Value: nil,
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
								b, err := marshal(Response{
									Call:  req.Call,
									Value: nil,
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
								v, err := marshal(res[0].Interface())
								if err != nil {
									setErr(err)

									return
								}

								b, err := marshal(Response{
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
							v, err := marshal(res[0].Interface())
							if err != nil {
								setErr(err)

								return
							}

							if res[1].Interface() == nil {
								b, err := marshal(Response{
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
								b, err := marshal(Response{
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

				var res Response
				if err := unmarshal(b, &res); err != nil {
					setErr(err)

					return
				}

				if strings.TrimSpace(res.Err) != "" {
					err = errors.New(res.Err)
				}

				go func() {
					if ok := responseResolver.Publish(res.Call, callResponse{res.Value, err, false}, r.timeout); !ok {
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

func (r Registry[R]) ForRemotes(
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
