package mockup

import (
	"encoding/json"
	"log"
	"net"
	"reflect"

	"github.com/google/uuid"
)

type Registry[L any, R any] struct {
	local  L
	remote R
}

func NewRegistry[L any, R any](local L, remote R) *Registry[L, R] {
	return &Registry[L, R]{local, remote}
}

func (r Registry[L, R]) Listen(lis net.Listener) error {
	// for {
	// 	conn, err := lis.Accept()
	// 	if err != nil {
	// 		log.Println("could not accept connection, continuing:", err)

	// 		continue
	// 	}

	// 	log.Println(conn)

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

			// TODO: Write to peer connection, then wait for response and return it (see registry.go for the unmarshalling of the response and the JS implementation on how to handle responses - use the registry context.Context to abort if there is no response)

			log.Println(string(b))

			return []reflect.Value{
				reflect.ValueOf(float64(25)),
			}
		})

		remote.FieldByName(functionField.Name).Set(fn)
	}
	// }

	return nil
}

func (r Registry[L, R]) Connect(conn net.Conn) error {
	return nil
}
