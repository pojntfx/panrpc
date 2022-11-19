package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net"
	"os"

	"github.com/pojntfx/dudirekta/pkg/rpc"
)

type local struct{}

func (s *local) Println(ctx context.Context, msg string) {
	log.Println("Printing message", msg, "for peer with ID", rpc.GetRemoteID(ctx))

	fmt.Println(msg)
}

type remote struct {
	Increment func(ctx context.Context, delta int64) int64
}

func main() {
	addr := flag.String("addr", "localhost:1337", "Listen or remote address")
	listen := flag.Bool("listen", false, "Whether to allow connecting to peers by listening or dialing")

	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := rpc.NewRegistry(
		&local{},
		remote{},
		ctx,
	)

	go func() {
		log.Println(`Enter one of the following letters followed by <ENTER> to run a function on the remote(s):

- a: Increment remote counter by one
- b: Decrement remote counter by one`)

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
					log.Println(peer.Increment(ctx, 1))
				case "b\n":
					log.Println(peer.Increment(ctx, -1))
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

		log.Println("Listening on", lis.Addr())

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

		log.Println("Connected to", conn.RemoteAddr())

		if err := registry.Link(conn); err != nil {
			panic(err)
		}
	}
}
