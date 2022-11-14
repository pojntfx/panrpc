package mockup

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"reflect"
	"sync"

	"github.com/google/uuid"
	"github.com/teivah/broadcast"
)

type response struct {
	id     string
	values []json.RawMessage
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

			clients++

			log.Printf("%v clients connected", clients)

			defer func() {
				clients--

				if err := recover(); err != nil {
					log.Printf("Client disconnected with error: %v", err)
				}

				log.Printf("%v clients connected", clients)
			}()

			responseResolver := broadcast.NewRelay[response]()
			go func() {
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

					if isCall {
						// TODO: Dispatch call to local struct
						continue
					}

					var id string
					if err := json.Unmarshal(res[1], &id); err != nil {
						panic(err)
					}

					responseResolver.Broadcast(response{id, res[2:]})
				}
			}()

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

					res := make(chan []json.RawMessage)
					go func() {
						for msg := range l.Ch() {
							if msg.id == callID {
								break
							}
						}

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
