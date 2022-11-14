package mockup

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"reflect"
	"sync"

	"github.com/google/uuid"
)

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

			clients++

			log.Printf("%v clients connected", clients)

			defer func() {
				clients--

				if err := recover(); err != nil {
					log.Printf("Client disconnected with error: %v", err)
				}

				log.Printf("%v clients connected", clients)
			}()

			remote := reflect.New(reflect.ValueOf(r.remote).Type()).Elem()

			for i := 0; i < remote.NumField(); i++ {
				functionField := remote.Type().Field(i)
				functionType := functionField.Type

				fn := reflect.MakeFunc(functionType, func(args []reflect.Value) (results []reflect.Value) {
					cmd := []any{true, uuid.NewString(), functionField.Name}

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

					res := make(chan []json.RawMessage)
					go func() {
						b, err := json.Marshal(25)
						if err != nil {
							panic(err)
						}

						res <- []json.RawMessage{json.RawMessage(b)}
					}()

					if _, err := conn.Write(b); err != nil {
						panic(err)
					}

					returnValues := []reflect.Value{}
					select {
					case rawReturnValues := <-res:
						for i, rawReturnValue := range rawReturnValues {
							returnValue := reflect.New(functionType.Out(i))

							if err := json.Unmarshal(rawReturnValue, returnValue.Interface()); err != nil {
								panic(err)
							}

							returnValues = append(returnValues, returnValue.Elem())
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
		}()
	}
}

func (r Registry[R]) Connect(conn net.Conn) error {
	return nil
}

func (r Registry[R]) Peers() map[string]R {
	return r.remotes
}
