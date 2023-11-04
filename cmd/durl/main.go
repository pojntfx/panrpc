package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
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
	"github.com/pojntfx/dudirekta/pkg/rpc"
	"nhooyr.io/websocket"
)

var (
	errMissingURL      = errors.New("missing URL")
	errMissingArgs     = errors.New("missing args")
	errMissingFunction = errors.New("missing function")
)

type reducedResponse struct {
	Value any    `json:"value"`
	Err   string `json:"err"`
}

func main() {
	flag.Usage = func() {
		bin := filepath.Base(os.Args[0])

		fmt.Fprintf(os.Stderr, `Like cURL, but for dudirekta: Command-line tool for interacting with dudirekta servers

Usage of %v:
	%v [flags] <(ws|wss|tcp|tls)://host:port/function> <[args...]>

Example:
	%v wss://jarvis.fel.p8.lu/ToggleLights '["token", { "kitchen": true, "bathroom": false }]'

Flags:
`, bin, bin, bin)

		flag.PrintDefaults()
	}

	listen := flag.Bool("listen", false, "Whether to connect to remotes by listening or dialing")
	timeout := flag.Duration("timeout", time.Second*10, "Time to wait for a response to a call")

	cert := flag.String("cert", "", "TLS certificate")
	key := flag.String("key", "", "TLS key")
	verify := flag.Bool("verify", true, "Whether to verify TLS peer certificates")

	verbose := flag.Bool("verbose", false, "Whether to enable verbose logging")

	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var (
		flagArgs = flag.Args()

		wsEnabled  = false
		tlsEnabled = false
		addr       = ""

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
	if tlsEnabled && strings.TrimSpace(*cert) != "" && strings.TrimSpace(*key) != "" {
		cert, err := tls.LoadX509KeyPair(*cert, *key)
		if err != nil {
			panic(err)
		}

		tlsConfig = &tls.Config{
			Certificates:       []tls.Certificate{cert},
			InsecureSkipVerify: *verify,
		}
	}

	callID := uuid.NewString()

	var (
		conn net.Conn
		err  error
	)
	if *listen {
		var lis net.Listener
		if tlsConfig == nil {
			lis, err = net.Listen("tcp", addr)
			if err != nil {
				panic(err)
			}
		} else {
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

			conn = <-connChan
		} else {
			conn, err = lis.Accept()
			if err != nil {
				panic(err)
			}
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

			conn = websocket.NetConn(ctx, c, websocket.MessageText)
		} else {
			if tlsConfig == nil {
				conn, err = net.Dial("tcp", addr)
				if err != nil {
					panic(err)
				}
			} else {
				conn, err = tls.Dial("tcp", addr, tlsConfig)
				if err != nil {
					panic(err)
				}
			}
		}
	}
	defer conn.Close()

	if *verbose {
		log.Println("Connected to", conn.RemoteAddr())
	}

	var req json.RawMessage
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
		log.Fatal(rpc.ErrCallTimedOut)

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
