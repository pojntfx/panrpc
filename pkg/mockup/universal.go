package mockup

import (
	"context"
	"encoding/json"
	"log"
	"net"
	"reflect"

	"github.com/google/uuid"
)

type Registry[L any, R any] struct {
	local  L
	remote R

	ctx context.Context
}

func NewRegistry[L any, R any](local L, remote R, ctx context.Context) *Registry[L, R] {
	return &Registry[L, R]{local, remote, ctx}
}

func (r Registry[L, R]) Listen(lis net.Listener) error {
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

			remote := reflect.
				ValueOf(r.remote).
				Elem()

			for i := 0; i < remote.NumField(); i++ {
				functionField := remote.Type().Field(i)
				functionType := functionField.Type

				fn := reflect.MakeFunc(functionType, func(args []reflect.Value) (results []reflect.Value) {
					cmd := []any{true, uuid.NewString(), functionField.Name}

					cmdArgs := []any{}
					for _, arg := range args {
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
							returnValue := reflect.New(functionType.In(i))

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
		}()
	}
}

func (r Registry[L, R]) Connect(conn net.Conn) error {
	return nil
}
