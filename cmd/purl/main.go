package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pojntfx/panrpc/pkg/rpc"
	"github.com/pojntfx/weron/pkg/wrtcconn"
	"github.com/rs/zerolog"
	"nhooyr.io/websocket"
)

var (
	errMissingURL       = errors.New("missing URL")
	errMissingArgs      = errors.New("missing args")
	errMissingFunction  = errors.New("missing function")
	errMissingPassword  = errors.New("missing password")
	errMissingKey       = errors.New("missing key")
	errMissingCommunity = errors.New("missing community")
	errMissingChannel   = errors.New("missing channel")
	errCallTimedOut     = errors.New("call timed out")
)

type reducedResponse struct {
	Value any    `json:"value"`
	Err   string `json:"err"`
}

func main() {
	flag.Usage = func() {
		bin := filepath.Base(os.Args[0])

		fmt.Fprintf(os.Stderr, `Like cURL, but for panrpc: Command-line tool for interacting with panrpc servers

Usage of %v:
	%v [flags] <(ws|wss|tcp|tls|weron)://(host:port/function|password:key@community/channel[/remote]/function)> <[args...]>

Examples:
	%v wss://manager.house.example.com/ToggleLights '["token", { "kitchen": true, "bathroom": false }]'
	%v weron://mypass:mykey@example.com/house/manager/ToggleLights '["token", { "kitchen": true, "bathroom": false }]'

Flags:
`, bin, bin, bin, bin)

		flag.PrintDefaults()
	}

	listen := flag.Bool("listen", false, "Whether to connect to remotes by listening or dialing (ignored for weron://)")
	timeout := flag.Duration("timeout", time.Second*10, "Time to wait for a response to a call")

	tlsCert := flag.String("tls-cert", "", "TLS certificate (only valid for wss:// and tls://)")
	tlsKey := flag.String("tls-key", "", "TLS key (only valid for wss:// and tls://)")
	tlsVerify := flag.Bool("tls-verify", true, "Whether to verify TLS peer certificates (only valid for wss:// and tls://)")

	weronSignaler := flag.String("weron-signaler", "wss://weron.up.railway.app/", "Signaler address (only valid for weron://)")
	weronICE := flag.String("weron-ice", "stun:stun.l.google.com:19302", "Comma-separated list of STUN servers (in format stun:host:port) and TURN servers to use (in format username:credential@turn:host:port) (i.e. username:credential@turn:global.turn.twilio.com:3478?transport=tcp) (only valid for weron://)")
	weronForceRelay := flag.Bool("weron-force-relay", false, "Force usage of TURN servers (only valid for weron://)")

	verbose := flag.Bool("verbose", false, "Whether to enable verbose logging")

	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if *verbose {
		zerolog.SetGlobalLevel(zerolog.TraceLevel)
	} else {
		zerolog.SetGlobalLevel(zerolog.InfoLevel)
	}

	var (
		flagArgs = flag.Args()

		wsEnabled    = false
		tlsEnabled   = false
		weronEnabled = false

		addr = ""

		password  = ""
		key       = ""
		community = ""
		channel   = ""
		remote    = ""

		callFunctionName = ""
		callArgsRaw      = ""
	)
	switch len(flagArgs) {
	case 1:
		flag.Usage()

		panic(errMissingArgs)

	case 2:
		u, err := url.Parse(flag.Arg(0))
		if err != nil {
			panic(err)
		}
		p := u.Port()

		switch u.Scheme {
		case "ws":
			wsEnabled = true
			tlsEnabled = false

			if p == "" {
				p = "80"
			}

		case "wss":
			wsEnabled = true
			tlsEnabled = true

			if p == "" {
				p = "443"
			}

		case "tcp":
			wsEnabled = false
			tlsEnabled = false

			if p == "" {
				p = "80"
			}

		case "tls":
			wsEnabled = false
			tlsEnabled = true

			if p == "" {
				p = "443"
			}

		case "weron":
			weronEnabled = true

			password = u.User.Username()
			if password == "" {
				panic(errMissingPassword)
			}

			key, _ = u.User.Password()
			if key == "" {
				panic(errMissingKey)
			}

			community = u.Hostname()
			if community == "" {
				panic(errMissingCommunity)
			}

			parts := strings.Split(u.Path, "/") // Indexes are off by one

			if len(parts) == 2 {
				panic(errMissingChannel)
			}
			channel = parts[1]

			if len(parts) > 3 {
				remote = parts[2]
			}
		}
		addr = net.JoinHostPort(u.Hostname(), p)

		callFunctionName = path.Base(u.Path)
		callArgsRaw = flag.Arg(1)

	default:
		flag.Usage()

		panic(errMissingURL)
	}

	if strings.TrimSpace(callFunctionName) == "" {
		flag.Usage()

		panic(errMissingFunction)
	}

	callArgs := []any{}
	if err := json.Unmarshal([]byte(callArgsRaw), &callArgs); err != nil {
		panic(err)
	}

	callArgsEncoded := []json.RawMessage{}
	for _, callArg := range callArgs {
		callArgEncoded, err := json.Marshal(callArg)
		if err != nil {
			panic(err)
		}

		callArgsEncoded = append(callArgsEncoded, callArgEncoded)
	}

	var tlsConfig *tls.Config
	if tlsEnabled && strings.TrimSpace(*tlsCert) != "" && strings.TrimSpace(*tlsKey) != "" {
		cert, err := tls.LoadX509KeyPair(*tlsCert, *tlsKey)
		if err != nil {
			panic(err)
		}

		tlsConfig = &tls.Config{
			Certificates:       []tls.Certificate{cert},
			InsecureSkipVerify: *tlsVerify,
		}
	}

	callID := uuid.NewString()

	var conn io.ReadWriteCloser
	if weronEnabled {
		u, err := url.Parse(*weronSignaler)
		if err != nil {
			panic(err)
		}

		q := u.Query()
		q.Set("community", community)
		q.Set("password", password)
		u.RawQuery = q.Encode()

		adapter := wrtcconn.NewAdapter(
			u.String(),
			key,
			strings.Split(*weronICE, ","),
			[]string{channel},
			&wrtcconn.AdapterConfig{
				Timeout:    *timeout,
				ForceRelay: *weronForceRelay,
				OnSignalerReconnect: func() {
					if *verbose {
						log.Println("Reconnecting to signaler with address", *weronSignaler)
					}
				},
			},
			ctx,
		)

		ids, err := adapter.Open()
		if err != nil {
			panic(err)
		}
		defer adapter.Close()

	l:
		for {
			select {
			case <-ctx.Done():
				if err := ctx.Err(); err != context.Canceled {
					panic(err)
				}

				return
			case rid := <-ids:
				if *verbose {
					log.Println("Listening on", rid)
				}
			case r := <-adapter.Accept():
				if remote == "" || remote == r.PeerID {
					if *verbose {
						log.Println("Connected to", r.PeerID)
					}

					conn = r.Conn

					break l
				}
			}
		}
	} else if *listen {
		var lis net.Listener
		if tlsConfig == nil {
			var err error
			lis, err = net.Listen("tcp", addr)
			if err != nil {
				panic(err)
			}
		} else {
			var err error
			lis, err = tls.Listen("tcp", addr, tlsConfig)
			if err != nil {
				panic(err)
			}
		}
		defer lis.Close()

		if *verbose {
			log.Println("Listening on", lis.Addr())
		}

		if wsEnabled {
			connChan := make(chan net.Conn)

			go http.Serve(lis, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				switch r.Method {
				case http.MethodGet:
					c, err := websocket.Accept(w, r, &websocket.AcceptOptions{
						OriginPatterns: []string{"*"},
					})
					if err != nil {
						panic(err)
					}

					connChan <- websocket.NetConn(ctx, c, websocket.MessageText)
				default:
					w.WriteHeader(http.StatusMethodNotAllowed)
				}
			}))

			c := <-connChan

			if *verbose {
				log.Println("Connected to", c.RemoteAddr())
			}

			conn = c
		} else {
			c, err := lis.Accept()
			if err != nil {
				panic(err)
			}

			if *verbose {
				log.Println("Connected to", c.RemoteAddr())
			}

			conn = c
		}

	} else {
		if wsEnabled {
			scheme := "ws://"
			if tlsEnabled {
				scheme = "wss://"
			}

			u, err := url.Parse(scheme + addr)
			if err != nil {
				panic(err)
			}

			c, _, err := websocket.Dial(ctx, u.String(), &websocket.DialOptions{
				HTTPClient: &http.Client{
					Transport: &http.Transport{
						TLSClientConfig: tlsConfig,
					},
				},
			})
			if err != nil {
				panic(err)
			}

			cc := websocket.NetConn(ctx, c, websocket.MessageText)

			if *verbose {
				log.Println("Connected to", cc.RemoteAddr())
			}

			conn = cc
		} else {
			if tlsConfig == nil {
				c, err := net.Dial("tcp", addr)
				if err != nil {
					panic(err)
				}

				if *verbose {
					log.Println("Connected to", c.RemoteAddr())
				}

				conn = c
			} else {
				c, err := tls.Dial("tcp", addr, tlsConfig)
				if err != nil {
					panic(err)
				}

				if *verbose {
					log.Println("Connected to", c.RemoteAddr())
				}

				conn = c
			}
		}
	}
	defer conn.Close()

	var (
		req json.RawMessage
		err error
	)
	req, err = json.Marshal(rpc.Request[json.RawMessage]{
		Call:     callID,
		Function: callFunctionName,
		Args:     callArgsEncoded,
	})
	if err != nil {
		panic(err)
	}

	if err := json.NewEncoder(conn).Encode(rpc.Message[json.RawMessage]{
		Request: &req,
	}); err != nil {
		panic(err)
	}

	var (
		timeoutChan  = time.After(*timeout)
		responseChan = make(chan rpc.Response[json.RawMessage])
	)
	go func() {
		decoder := json.NewDecoder(conn)
		for {
			var msg rpc.Message[json.RawMessage]
			if err := decoder.Decode(&msg); err != nil {
				panic(err)
			}

			if msg.Response == nil {
				if *verbose {
					log.Println("Received request, skipping:", msg)
				}

				continue
			}

			var response rpc.Response[json.RawMessage]
			if err := json.Unmarshal(*msg.Response, &response); err != nil {
				panic(err)
			}

			responseChan <- response

			break
		}
	}()

	select {
	case <-timeoutChan:
		log.Fatal(errCallTimedOut)

	case response := <-responseChan:
		var value any
		if err := json.Unmarshal(response.Value, &value); err != nil {
			panic(err)
		}

		if err := json.NewEncoder(os.Stdout).Encode(reducedResponse{
			Value: value,
			Err:   response.Err,
		}); err != nil {
			panic(err)
		}
	}
}