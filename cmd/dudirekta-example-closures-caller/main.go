package main

import (
	"bufio"
	"context"
	"flag"
	"log"
	"net"
	"os"
	"time"

	"github.com/pojntfx/dudirekta/pkg/closures"
	"github.com/pojntfx/dudirekta/pkg/rpc"
)

type local struct {
	*closures.ClosureManager
}

type remote struct {
	Iterate func(
		ctx context.Context,
		length int,
		onIterationClosureID string,
	) (int, error)
}

func Iterate(caller *local, callee remote, ctx context.Context, length int, onIteration func(i int) error) (int, error) {
	onIterationClosureID, freeClosure, err := closures.RegisterClosure(caller.ClosureManager, onIteration)
	if err != nil {
		return -1, err
	}
	defer freeClosure()

	return callee.Iterate(ctx, length, onIterationClosureID)
}

func main() {
	addr := flag.String("addr", "localhost:1337", "Remote address")

	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service := &local{closures.NewClosureManager()}

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
