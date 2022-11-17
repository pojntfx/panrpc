package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/pojntfx/dudirekta/pkg/mockup"
)

type local struct{}

func (s local) Divide(ctx context.Context, divident, divisor float64) (quotient float64, err error) {
	log.Printf("Dividing for remote %v, divident %v and divisor %v", mockup.GetRemoteID(ctx), divident, divisor)

	if divisor == 0 {
		return -1, errors.New("could not divide by zero")
	}

	return divident / divisor, nil
}

type remote struct {
	Multiply      func(ctx context.Context, multiplicant, multiplier float64) (product float64)
	PrintString   func(ctx context.Context, msg string)
	ValidateEmail func(ctx context.Context, email string) error
	ParseJSON     func(ctx context.Context, p []byte) (any, error)
}

func main() {
	addr := flag.String("addr", "localhost:1337", "Listen or remote address")
	listen := flag.Bool("listen", true, "Whether to allow connecting to peers by listening or dialing")

	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := mockup.NewRegistry(
		&local{},
		remote{},
		ctx,
	)

	go func() {
		log.Println(`Enter one of the following letters followed by <ENTER> to run a function on the remote:

- a: Print "Hello from remote!" on remote's stdout
- b: Validate email "anne@example.com"
- c: Validate email "asdf" (will throw an error)
- d: Parse JSON "{"name": "Jane"}"
- e: Parse JSON "{"n" (will throw an error)`)

		stdin := bufio.NewReader(os.Stdin)

		for {
			line, err := stdin.ReadString('\n')
			if err != nil {
				panic(err)
			}

			for peerID, peer := range registry.Peers() {
				log.Println("Calling functions for peer with ID", peerID)

				switch line {
				case "a\n":
					peer.PrintString(ctx, "Hello from remote!")
				case "b\n":
					if err := peer.ValidateEmail(ctx, "anne@example.com"); err != nil {
						log.Println("Got email validation error:", err)
					}
				case "c\n":
					if err := peer.ValidateEmail(ctx, "asdf"); err != nil {
						log.Println("Got email validation error:", err)
					}
				case "d\n":
					res, err := peer.ParseJSON(ctx, []byte(`{"name": "Jane"}`))
					if err != nil {
						log.Println("Got JSON parser error:", err)
					} else {
						fmt.Println(res)
					}
				case "e\n":
					res, err := peer.ParseJSON(ctx, []byte(`{"n`))
					if err != nil {
						log.Println("Got JSON parser error:", err)
					} else {
						fmt.Println(res)
					}
				default:
					log.Printf("Unknown letter %v, ignoring input", line)

					continue
				}
			}
		}
	}()

	if *listen {
		lis, err := net.Listen("tcp", *addr)
		if err != nil {
			panic(err)
		}
		defer lis.Close()

		log.Printf("Listening on dudirekta+%v://%v", lis.Addr().Network(), lis.Addr().String())

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

					if err := registry.Link(conn); err != nil {
						panic(err)
					}
				}()
			}()
		}

	} else {
		conn, err := net.Dial("tcp", *addr)
		if err != nil {
			panic(err)
		}
		defer conn.Close()

		log.Printf("Connected to dudirekta+%v://%v", conn.RemoteAddr().Network(), conn.RemoteAddr().String())

		if err := registry.Link(conn); err != nil {
			panic(err)
		}
	}
}
