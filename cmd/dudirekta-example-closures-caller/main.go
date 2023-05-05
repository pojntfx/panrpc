package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"log"
	"net"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/google/uuid"
	"github.com/pojntfx/dudirekta/pkg/rpc"
)

var (
	errorType = reflect.TypeOf((*error)(nil)).Elem()

	ErrNotAFunction     = errors.New("not a function")
	ErrInvalidArgsCount = errors.New("invalid argument count")
	ErrInvalidArg       = errors.New("invalid argument")
)

func CreateClosure(fn interface{}) (func(args ...interface{}) (interface{}, error), error) {
	functionType := reflect.ValueOf(fn).Type()

	if functionType.Kind() != reflect.Func {
		return nil, ErrNotAFunction
	}

	if functionType.NumOut() <= 0 || functionType.NumOut() > 2 {
		return nil, rpc.ErrInvalidReturn
	}

	if !functionType.Out(functionType.NumOut() - 1).Implements(errorType) {
		return nil, rpc.ErrInvalidReturn
	}

	return func(args ...interface{}) (interface{}, error) {
		if len(args) != functionType.NumIn() {
			return nil, ErrInvalidArgsCount
		}

		in := make([]reflect.Value, len(args))
		for i, arg := range args {
			if argType := reflect.TypeOf(arg); argType != functionType.In(i) {
				if argType.ConvertibleTo(functionType.In(i)) {
					in[i] = reflect.ValueOf(arg).Convert(functionType.In(i))
				} else {
					return nil, ErrInvalidArg
				}
			} else {
				in[i] = reflect.ValueOf(arg)
			}
		}

		out := reflect.ValueOf(fn).Call(in)
		if len(out) == 1 {
			if out[0].IsValid() && !out[0].IsNil() {
				return nil, out[0].Interface().(error)
			}

			return nil, nil
		}

		if out[1].IsValid() && !out[1].IsNil() {
			return out[0].Interface(), out[1].Interface().(error)
		}

		return out[0].Interface(), nil
	}, nil
}

type local struct {
	closuresLock sync.Mutex
	closures     map[string]func(args ...interface{}) (interface{}, error)
}

func (s *local) ResolveClosure(ctx context.Context, closureID string, args []interface{}) (interface{}, error) {
	s.closuresLock.Lock()
	closure, ok := s.closures[closureID]
	if !ok {
		s.closuresLock.Unlock()

		return nil, errors.New("could not find closure")
	}
	s.closuresLock.Unlock()

	return closure(args...)
}

type remote struct {
	Iterate func(
		ctx context.Context,
		length int,
		onIterationClosureID string,
	) (int, error)
}

func Iterate(caller *local, callee remote, ctx context.Context, length int, onIteration func(i int) error) (int, error) {
	fn, err := CreateClosure(onIteration)
	if err != nil {
		return -1, err
	}

	onIterationClosureID := uuid.New().String()

	caller.closuresLock.Lock()
	caller.closures[onIterationClosureID] = fn
	caller.closuresLock.Unlock()

	defer func() {
		caller.closuresLock.Lock()
		delete(caller.closures, onIterationClosureID)
		caller.closuresLock.Unlock()
	}()

	return callee.Iterate(ctx, length, onIterationClosureID)
}

func main() {
	addr := flag.String("addr", "localhost:1337", "Remote address")

	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service := &local{
		closuresLock: sync.Mutex{},
		closures:     map[string]func(args ...interface{}) (interface{}, error){},
	}

	clients := 0
	registry := rpc.NewRegistry(
		service,
		remote{},

		time.Second*10,
		ctx,
		&rpc.Options{
			ResponseBufferLen: rpc.DefaultResponseBufferLen,
			OnClientConnect: func(remoteID string) {
				clients++

				log.Printf("%v clients connected", clients)
			},
			OnClientDisconnect: func(remoteID string) {
				clients--

				log.Printf("%v clients connected", clients)
			},
		},
	)

	go func() {
		log.Println(`Enter one of the following letters followed by <ENTER> to run a function on the remote(s):

- a: Iterate over 5
- b: Iterate over 10`)

		stdin := bufio.NewReader(os.Stdin)

		for {
			line, err := stdin.ReadString('\n')
			if err != nil {
				panic(err)
			}

			for _, peer := range registry.Peers() {
				switch line {
				case "a\n":
					length, err := Iterate(service, peer, ctx, 5, func(i int) error {
						log.Println("In iteration", i)

						return nil
					})
					if err != nil {
						log.Println("Got error for Iterate func:", err)

						continue
					}

					log.Println(length)
				case "b\n":
					length, err := Iterate(service, peer, ctx, 10, func(i int) error {
						log.Println("In iteration", i)

						return nil
					})
					if err != nil {
						log.Println("Got error for Iterate func:", err)

						continue
					}

					log.Println(length)
				default:
					log.Printf("Unknown letter %v, ignoring input", line)

					continue
				}
			}
		}
	}()

	conn, err := net.Dial("tcp", *addr)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	log.Println("Connected to", conn.RemoteAddr())

	if err := registry.Link(conn); err != nil {
		panic(err)
	}
}
