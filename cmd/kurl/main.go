package main

import (
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net"
	"os"
	"strings"

	"github.com/google/uuid"
	"github.com/pojntfx/dudirekta/pkg/rpc"
)

var (
	errMissingFunctionName = errors.New("missing function name")
)

type reducedResponse struct {
	Value any    `json:"value"`
	Err   string `json:"err"`
}

func main() {
	raddr := flag.String("raddr", "localhost:1337", "Remote address")
	function := flag.String("function", "", "Remote function name to call")
	rawArgs := flag.String("args", `[]`, "JSON-encoded Array of arguments to call remote function with (i.e. `[1, \"Hello, world\", { \"nested:\": true }])")
	verbose := flag.Bool("verbose", false, "Whether to enable verbose logging")

	flag.Parse()

	conn, err := net.Dial("tcp", *raddr)
	if err != nil {
		panic(err)
	}
	defer conn.Close()

	if *verbose {
		log.Println("Connected to", conn.RemoteAddr())
	}

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

	callID := uuid.NewString()

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

		var value any
		if err := json.Unmarshal(response.Value, &value); err != nil {
			panic(err)
		}

		json.NewEncoder(os.Stdout).Encode(reducedResponse{
			Value: value,
			Err:   response.Err,
		})

		break
	}
}
