package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/pojntfx/dudirekta/pkg/rpc"
	"github.com/pojntfx/weron/pkg/wrtcconn"
	"github.com/rs/zerolog"
	"nhooyr.io/websocket"
)

var (
	errMissingCommunity = errors.New("missing community")
	errMissingPassword  = errors.New("missing password")
	errMissingKey       = errors.New("missing key")
)

type local struct{}

func (s local) Divide(ctx context.Context, divident, divisor float64) (quotient float64, err error) {
	log.Printf("Dividing for remote %v, divident %v and divisor %v", rpc.GetRemoteID(ctx), divident, divisor)

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
	transport := flag.String("transport", "tcp", "Transport to use (valid options are tcp, websockets or webrtc)")
	verbose := flag.Int("verbose", 5, "Verbosity level (0 is disabled, default is info, 7 is trace)")
	signaler := flag.String("signaler", "wss://weron.herokuapp.com/", "Signaler address")
	timeout := flag.Duration("timeout", time.Second*10, "Time to wait for connections")
	community := flag.String("community", "", "ID of community to join")
	password := flag.String("password", "", "Password for community")
	key := flag.String("key", "", "Encryption key for community")
	ice := flag.String("ice", "stun:stun.l.google.com:19302", "Comma-separated list of STUN servers (in format stun:host:port) and TURN servers to use (in format username:credential@turn:host:port) (i.e. username:credential@turn:global.turn.twilio.com:3478?transport=tcp)")
	forceRelay := flag.Bool("force-relay", false, "Force usage of TURN servers")

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

	switch *transport {
	case "tcp":
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

	case "websockets":
		if *listen {
			lis, err := net.Listen("tcp", *addr)
			if err != nil {
				panic(err)
			}
			defer lis.Close()

			log.Printf("Listening on dudirekta+%v://%v", "ws", lis.Addr().String())

			clients := 0

			if err := http.Serve(lis, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				clients++

				log.Printf("%v clients connected", clients)

				defer func() {
					clients--

					if err := recover(); err != nil {
						w.WriteHeader(http.StatusInternalServerError)

						log.Printf("Client disconnected with error: %v", err)
					}

					log.Printf("%v clients connected", clients)
				}()

				switch r.Method {
				case http.MethodGet:
					c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
						OriginPatterns: []string{"*"},
					})
					if err != nil {
						panic(err)
					}

					pings := time.NewTicker(time.Second / 2)
					defer pings.Stop()

					errs := make(chan error)
					go func() {
						for range pings.C {
							if err := c.Ping(ctx); err != nil {
								errs <- err

								return
							}
						}
					}()

					conn := websocket.NetConn(ctx, c, websocket.MessageText)
					defer conn.Close()

					go func() {
						if err := registry.Link(conn); err != nil {
							errs <- err

							return
						}
					}()

					if err := <-errs; err != nil {
						panic(err)
					}
				default:
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			})); err != nil {
				panic(err)
			}
		} else {
			c, _, err := websocket.Dial(ctx, "ws://"+*addr, nil)
			if err != nil {
				panic(err)
			}

			conn := websocket.NetConn(ctx, c, websocket.MessageText)
			defer conn.Close()

			log.Printf("Connected to dudirekta+%v://%v", "ws", conn.RemoteAddr().String())

			if err := registry.Link(conn); err != nil {
				panic(err)
			}
		}

	case "webrtc":
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
			panic(errMissingCommunity)
		}

		if strings.TrimSpace(*password) == "" {
			panic(errMissingPassword)
		}

		if strings.TrimSpace(*key) == "" {
			panic(errMissingKey)
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
			[]string{"dudirekta/" + *addr},
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

		log.Printf("Listening on dudirekta+%v://%v/dudirekta/%v", "webrtc", *signaler, *addr)

		clients := 0
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
				log.Println("Connected to signaler with address", *signaler, "and ID", rid)
			case peer := <-adapter.Accept():
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

					if err := registry.Link(peer.Conn); err != nil {
						panic(err)
					}
				}()
			}
		}
	}
}
