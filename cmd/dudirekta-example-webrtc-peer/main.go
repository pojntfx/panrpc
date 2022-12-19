package main

import (
	"bufio"
	"context"
	"flag"
	"fmt"
	"log"
	"net/url"
	"os"
	"strings"
	"sync/atomic"
	"time"

	"github.com/pojntfx/dudirekta/pkg/rpc"
	"github.com/pojntfx/weron/pkg/wrtcconn"
	"github.com/rs/zerolog"
)

type local struct {
	counter int64
}

func (s *local) Increment(ctx context.Context, delta int64) (int64, error) {
	log.Println("Incrementing counter by", delta, "for peer with ID", rpc.GetRemoteID(ctx))

	return atomic.AddInt64(&s.counter, delta), nil
}

func (s *local) Println(ctx context.Context, msg string) error {
	log.Println("Printing message", msg, "for peer with ID", rpc.GetRemoteID(ctx))

	fmt.Println(msg)

	return nil
}

type remote struct {
	Increment func(ctx context.Context, delta int64) (int64, error)
	Println   func(ctx context.Context, msg string) error
}

func main() {
	verbose := flag.Int("verbose", 5, "Verbosity level (0 is disabled, default is info, 7 is trace)")
	signaler := flag.String("signaler", "wss://weron.up.railway.app/", "Signaler address")
	timeout := flag.Duration("timeout", time.Second*10, "Time to wait for connections")
	community := flag.String("community", "", "ID of community to join")
	password := flag.String("password", "", "Password for community")
	key := flag.String("key", "", "Encryption key for community")
	ice := flag.String("ice", "stun:stun.l.google.com:19302", "Comma-separated list of STUN servers (in format stun:host:port) and TURN servers to use (in format username:credential@turn:host:port) (i.e. username:credential@turn:global.turn.twilio.com:3478?transport=tcp)")
	forceRelay := flag.Bool("force-relay", false, "Force usage of TURN servers")

	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clients := 0

	registry := rpc.NewRegistry(
		&local{},
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
		log.Println(`Enter one of the following letters followed by <ENTER> to run a function on the remote:

- a: Increment remote counter by one
- b: Decrement remote counter by one
- c: Print "Hello, world!"`)

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
					new, err := peer.Increment(ctx, 1)
					if err != nil {
						log.Println("Got error for Increment func:", err)

						continue
					}

					log.Println(new)
				case "b\n":
					new, err := peer.Increment(ctx, -1)
					if err != nil {
						log.Println("Got error for Increment func:", err)

						continue
					}

					log.Println(new)
				case "c\n":
					if err := peer.Println(ctx, "Hello, world!"); err != nil {
						log.Println("Got error for Println func:", err)

						continue
					}
				default:
					log.Printf("Unknown letter %v, ignoring input", line)

					continue
				}
			}
		}
	}()

	switch *verbose {
	case 0:
		zerolog.SetGlobalLevel(zerolog.Disabled)
	case 1:
		zerolog.SetGlobalLevel(zerolog.PanicLevel)
	case 2:
		zerolog.SetGlobalLevel(zerolog.FatalLevel)
	case 3:
		zerolog.SetGlobalLevel(zerolog.ErrorLevel)
	case 4:
		zerolog.SetGlobalLevel(zerolog.WarnLevel)
	case 5:
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	case 6:
		zerolog.SetGlobalLevel(zerolog.DebugLevel)
	default:
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	}

	if strings.TrimSpace(*community) == "" {
		panic("could not continue with empty community")
	}

	if strings.TrimSpace(*password) == "" {
		panic("could not continue with empty password")
	}

	if strings.TrimSpace(*key) == "" {
		panic("could not continue with empty key")
	}

	u, err := url.Parse(*signaler)
	if err != nil {
		panic(err)
	}

	q := u.Query()
	q.Set("community", *community)
	q.Set("password", *password)
	u.RawQuery = q.Encode()

	adapter := wrtcconn.NewAdapter(
		u.String(),
		*key,
		strings.Split(*ice, ","),
		[]string{"dudirekta/example/webrtc"},
		&wrtcconn.AdapterConfig{
			Timeout:    *timeout,
			ForceRelay: *forceRelay,
			OnSignalerReconnect: func() {
				log.Println("Reconnecting to signaler with address", *signaler)
			},
		},
		ctx,
	)

	ids, err := adapter.Open()
	if err != nil {
		panic(err)
	}
	defer adapter.Close()

	errs := make(chan error)
	for {
		select {
		case <-ctx.Done():
			if err := ctx.Err(); err != context.Canceled {
				panic(err)
			}

			return
		case err := <-errs:
			panic(err)
		case rid := <-ids:
			log.Println("Listening with ID", rid)
		case peer := <-adapter.Accept():
			log.Println("Connected to peer with ID", peer.PeerID)

			go func() {
				defer func() {
					_ = peer.Conn.Close()

					if err := recover(); err != nil {
						log.Printf("Client disconnected with error: %v", err)
					}
				}()

				if err := registry.Link(peer.Conn); err != nil {
					panic(err)
				}
			}()
		}
	}
}
