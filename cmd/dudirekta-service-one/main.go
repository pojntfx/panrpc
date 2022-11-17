package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net"
	"net/mail"
	"os"

	"github.com/pojntfx/dudirekta/pkg/mockup"
)

type local struct{}

func (s local) Multiply(ctx context.Context, multiplicant, multiplier float64) (product float64) {
	log.Printf("Multiplying for remote %v, multiplicant %v and multiplier %v", mockup.GetRemoteID(ctx), multiplicant, multiplier)

	return multiplicant * multiplier
}

func (s local) PrintString(ctx context.Context, msg string) {
	log.Printf("Printing string for remote %v and message %v", mockup.GetRemoteID(ctx), msg)

	fmt.Println(msg)
}

func (s local) ValidateEmail(ctx context.Context, email string) error {
	_, err := mail.ParseAddress(email)

	log.Printf("Validating email for remote %v and email %v", mockup.GetRemoteID(ctx), email)

	return err
}

func (s local) ParseJSON(ctx context.Context, p []byte) (any, error) {
	var output any
	if err := json.Unmarshal(p, &output); err != nil {
		return nil, err
	}

	return output, nil
}

type remote struct {
	Divide func(ctx context.Context, divident, divisor float64) (quotient float64, err error)
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

- a: Divide 50 by 2
- b: Divide 50 by 0 (will throw an error)`)

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
					quotient, err := peer.Divide(ctx, 50, 2)
					if err != nil {
						log.Println("Got division error:", err)
					} else {
						fmt.Println(quotient)
					}
				case "b\n":
					quotient, err := peer.Divide(ctx, 50, 0)
					if err != nil {
						log.Println("Got division error:", err)
					} else {
						fmt.Println(quotient)
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
