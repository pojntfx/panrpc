package main

import (
	"encoding/json"
	"errors"
	"flag"
	"log"
	"net"
	"net/http"
	"time"

	"github.com/pojntfx/dudirekta/pkg/rpc"
)

func main() {
	laddr := flag.String("laddr", ":1337", "Listen address")
	heartbeat := flag.Duration("heartbeat", time.Second*10, "Interval in which to send keepalive heartbeats")

	registry := rpc.NewRegistry(*heartbeat, &rpc.Callbacks{
		OnReceivePong: func() {
			log.Println("Received pong from client")
		},
		OnSendingPing: func() {
			log.Println("Sending ping to client")
		},
		OnFunctionCall: func(requestID, functionName string, functionArgs []json.RawMessage) {
			log.Printf("Got request ID %v for function %v with args %v", requestID, functionName, functionArgs)
		},
	})

	if err := registry.Bind("exampleReturnStringAndError", func(msg string, returnError bool) (string, error) {
		if returnError {
			return "", errors.New("test error")
		}

		return msg, nil
	}); err != nil {
		panic(err)
	}

	listener, err := net.Listen("tcp", *laddr)
	if err != nil {
		panic(err)
	}
	defer listener.Close()

	log.Println("Listening on", *laddr)

	clients := 0
	if err := http.Serve(listener, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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
			if err := registry.HandlerFunc(w, r); err != nil {
				panic(err)
			}
		default:
			w.WriteHeader(http.StatusMethodNotAllowed)
		}
	})); err != nil {
		panic(err)
	}
}
