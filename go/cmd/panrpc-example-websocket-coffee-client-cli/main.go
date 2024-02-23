package main

import (
	"bufio"
	"context"
	"encoding/json"
	"flag"
	"log"
	"os"

	"github.com/pojntfx/panrpc/go/pkg/rpc"
	"nhooyr.io/websocket"
)

type remoteControl struct{}

func (s *remoteControl) SetCoffeeMachineBrewing(ctx context.Context, brewing bool) error {
	if brewing {
		log.Println("Coffee machine is now brewing")
	} else {
		log.Println("Coffee machine has stopped brewing")
	}

	return nil
}

type coffeeMachine struct {
	BrewCoffee func(
		ctx context.Context,
		variant string,
		size int,
		onProgress func(ctx context.Context, percentage int) error,
	) (int, error)
}

func main() {
	flag.Parse()

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	clients := 0

	registry := rpc.NewRegistry[coffeeMachine, json.RawMessage](
		&remoteControl{},

		ctx,

		&rpc.Options{
			OnClientConnect: func(remoteID string) {
				clients++

				log.Printf("%v coffee machines connected", clients)
			},
			OnClientDisconnect: func(remoteID string) {
				clients--

				log.Printf("%v coffee machines connected", clients)
			},
		},
	)

	go func() {
		log.Println(`Enter one of the following numbers followed by <ENTER> to brew a coffee:

- 1: Brew small Cafè Latte
- 2: Brew large Cafè Latte

- 3: Brew small Americano
- 4: Brew large Americano`)

		stdin := bufio.NewReader(os.Stdin)

		for {
			line, err := stdin.ReadString('\n')
			if err != nil {
				panic(err)
			}

			if err := registry.ForRemotes(func(remoteID string, remote coffeeMachine) error {
				switch line {
				case "1\n":
					fallthrough
				case "2\n":
					res, err := remote.BrewCoffee(
						ctx,
						"latte",
						func() int {
							if line == "1" {
								return 100
							} else {
								return 200
							}
						}(),
						func(ctx context.Context, percentage int) error {
							log.Printf(`Brewing Cafè Latte ... %v%% done`, percentage)

							return nil
						},
					)
					if err != nil {
						log.Println("Couldn't brew Cafè Latte:", err)

						return nil
					}

					log.Println("Remaining water:", res, "ml")

				case "3\n":
					fallthrough
				case "4\n":
					res, err := remote.BrewCoffee(
						ctx,
						"americano",
						func() int {
							if line == "1" {
								return 100
							} else {
								return 200
							}
						}(),
						func(ctx context.Context, percentage int) error {
							log.Printf(`Brewing Americano ... %v%% done`, percentage)

							return nil
						},
					)
					if err != nil {
						log.Println("Couldn't brew Americano:", err)

						return nil
					}

					log.Println("Remaining water:", res, "ml")

				default:
					log.Printf("Unknown letter %v, ignoring input", line)

					return nil
				}

				return nil
			}); err != nil {
				panic(err)
			}
		}
	}()

	c, _, err := websocket.Dial(ctx, "ws://127.0.0.1:1337", nil)
	if err != nil {
		panic(err)
	}

	conn := websocket.NetConn(ctx, c, websocket.MessageText)
	defer conn.Close()

	log.Println("Connected to localhost:1337")

	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	if err := registry.LinkStream(
		func(v rpc.Message[json.RawMessage]) error {
			return encoder.Encode(v)
		},
		func(v *rpc.Message[json.RawMessage]) error {
			return decoder.Decode(v)
		},

		func(v any) (json.RawMessage, error) {
			b, err := json.Marshal(v)
			if err != nil {
				return nil, err
			}

			return json.RawMessage(b), nil
		},
		func(data json.RawMessage, v any) error {
			return json.Unmarshal([]byte(data), v)
		},
	); err != nil {
		panic(err)
	}
}
