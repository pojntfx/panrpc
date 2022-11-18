package main

import (
	"bufio"
	"context"
	"flag"
	"log"
	"net"
	"os"
	"os/exec"
	"sync/atomic"
	"time"

	"github.com/pojntfx/dudirekta/pkg/rpc"
)

type local struct {
	counter int64
}

func (s local) GetTime(ctx context.Context) time.Time {
	log.Println("Getting time for peer with ID", rpc.GetRemoteID(ctx))

	return time.Now()
}

func (s *local) Increment(ctx context.Context, delta int64) int64 {
	log.Println("Incrementing counter by", delta, "for peer with ID", rpc.GetRemoteID(ctx))

	return atomic.AddInt64(&s.counter, delta)
}

func (s *local) OpenHomeFolder(ctx context.Context) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}

	return exec.Command("xdg-open", home).Run()
}

type remote struct {
	GetTime        func(ctx context.Context) time.Time
	Increment      func(ctx context.Context, delta int64) int64
	OpenHomeFolder func(ctx context.Context) error
}

func main() {
	addr := flag.String("addr", "localhost:1337", "Listen or remote address")
	listen := flag.Bool("listen", true, "Whether to allow connecting to peers by listening or dialing")

	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	registry := rpc.NewRegistry(
		&local{},
		remote{},
		ctx,
	)

	go func() {
		log.Println(`Enter one of the following letters followed by <ENTER> to run a function on the remote:

- a: Get remote server's current time
- b: Increment remote counter by one
- c: Decrement remote counter by one
- d: Open home folder in file explorer on remote`)

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
					log.Println(peer.GetTime(ctx))
				case "b\n":
					log.Println(peer.Increment(ctx, 1))
				case "c\n":
					log.Println(peer.Increment(ctx, -1))
				case "d\n":
					log.Println(peer.OpenHomeFolder(ctx))
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
