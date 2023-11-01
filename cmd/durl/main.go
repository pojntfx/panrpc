package main

import (
	"context"
	"crypto/tls"
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/pojntfx/dudirekta/pkg/rpc"
	"nhooyr.io/websocket"
)

var (
	errMissingFunctionName = errors.New("missing function name")
)

type reducedResponse struct {
	Value any    `json:"value"`
	Err   string `json:"err"`
}

func main() {
	addr := flag.String("addr", "localhost:1337", "Listen or remote address")
	listen := flag.Bool("listen", false, "Whether to allow connecting to remotes by listening or dialing")
	function := flag.String("function", "", "Remote function name to call")
	rawArgs := flag.String("args", `[]`, "JSON-encoded Array of arguments to call remote function with (i.e. `[1, \"Hello, world\", { \"nested:\": true }])")
	timeout := flag.Duration("timeout", time.Second*10, "Time to wait for a response")
	ws := flag.Bool("websocket", false, "Whether to use WebSockets instead of TCP")

	tlsEnabled := flag.Bool("tls-enabled", false, "Whether to use TLS/WebSocket Secure")
	tlsCert := flag.String("tls-cert", "", "TLS certificate")
	tlsKey := flag.String("tls-key", "", "TLS key")
	tlsVerify := flag.Bool("tls-verify", true, "Whether to verify TLS peer certificates")

	verbose := flag.Bool("verbose", false, "Whether to enable verbose logging")

	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if strings.TrimSpace(*function) == "" {
		panic(errMissingFunctionName)
	}

	args := []any{}
	if err := json.Unmarshal([]byte(*rawArgs), &args); err != nil {
		panic(err)
	}

	encodedArgs := []json.RawMessage{}
	for _, arg := range args {
		encodedArg, err := json.Marshal(arg)
		if err != nil {
			panic(err)
		}

		encodedArgs = append(encodedArgs, encodedArg)
	}

	var tlsConfig *tls.Config
	if *tlsEnabled && strings.TrimSpace(*tlsCert) != "" && strings.TrimSpace(*tlsKey) != "" {
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

	var (
		conn net.Conn
		err  error
	)
	if *listen {
		var lis net.Listener
		if tlsConfig == nil {
			lis, err = net.Listen("tcp", *addr)
			if err != nil {
				panic(err)
			}
		} else {
			lis, err = tls.Listen("tcp", *addr, tlsConfig)
			if err != nil {
				panic(err)
			}
		}
		defer lis.Close()

		if *verbose {
			log.Println("Listening on", lis.Addr())
		}

		if *ws {
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
		if *ws {
			c, _, err := websocket.Dial(ctx, *addr, &websocket.DialOptions{
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
				conn, err = net.Dial("tcp", *addr)
				if err != nil {
					panic(err)
				}
			} else {
				conn, err = tls.Dial("tcp", *addr, tlsConfig)
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
		Function: *function,
		Args:     encodedArgs,
	})
	if err != nil {
		panic(err)
	}

	encoder := json.NewEncoder(conn)
	if err := encoder.Encode(rpc.Message[json.RawMessage]{
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
