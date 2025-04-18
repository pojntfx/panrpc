<img alt="Project icon" style="vertical-align: middle;" src="./docs/icon.svg" width="128" height="128" align="left">

# panrpc

Language-, transport- and serialization-agnostic RPC framework with remote closure support that allows exposing and calling functions on both clients and servers.

[![hydrun CI](https://github.com/pojntfx/panrpc/actions/workflows/hydrun.yaml/badge.svg)](https://github.com/pojntfx/panrpc/actions/workflows/hydrun.yaml)
![Go Version](https://img.shields.io/badge/go%20version-%3E=1.18-61CFDD.svg)
[![Go Reference](https://pkg.go.dev/badge/github.com/pojntfx/panrpc/go.svg)](https://pkg.go.dev/github.com/pojntfx/panrpc/go)
[![npm CI](https://github.com/pojntfx/panrpc/actions/workflows/npm.yaml/badge.svg)](https://github.com/pojntfx/panrpc/actions/workflows/npm.yaml)
[![npm: @pojntfx/panrpc](https://img.shields.io/npm/v/@pojntfx/panrpc)](https://www.npmjs.com/package/@pojntfx/panrpc)
[![TypeScript docs](https://img.shields.io/badge/TypeScript%20-docs-blue.svg)](https://pojntfx.github.io/panrpc)
[![Matrix](https://img.shields.io/matrix/panrpc:matrix.org)](https://matrix.to/#/#panrpc:matrix.org?via=matrix.org)

## Overview

panrpc is a flexible high-performance RPC framework designed to work in almost any environment with advanced features such as remote closures and bidirectional RPC calls.

It enables you to:

- **Transparently call and expose RPCs in many languages**: Thanks to it's use of reflection, panrpc doesn't require you to learn a DSL or run a code generator. RPCs are defined and called as local functions, and its [simple protocol](#protocol) means that [multiple languages](#examples) are supported and adding support for new ones is simple.
- **Work with any transport layer**: Instead of being restricted to one transport layer (like TCP or WebSockets for most RPC frameworks), panrpc depends only on the semantics of a [stream](https://pkg.go.dev/github.com/pojntfx/panrpc/go/pkg/rpc#LinkStream) or a [message](https://pkg.go.dev/github.com/pojntfx/panrpc/go/pkg/rpc#LinkMessage), meaning it works over everything from TCP, WebSockets, UNIX sockets, WebRTC, Valkey/Redis, NATS and more.
- **Work with any serializer**: Instead of being restricted to one serialization framework (like Protobuf or JSON for most RPC frameworks), panrpc can use any user-defined serializer that supports streaming encode/decode, such as JSON, CBOR and others.
- **Call RPCs on both clients and servers**: Unlike most RPC frameworks, which only allow you to call a server's RPCs from a client, panrpc can also work with the reverse configuration (where the server calls RPCs exposed by the client) or both at the same time.
- **Pass closures to RPCs**: You can transparently pass closures and callbacks to RPCs as function parameters, and they will be called by the RPC just like if it were a local function call.

## Installation

### Library

You can add panrpc to your <img alt="Go" src="https://cdn.simpleicons.org/go" style="vertical-align: middle;" height="20" width="20" /> **Go** project by running the following:

```shell
$ go get github.com/pojntfx/panrpc/go/...@latest
```

For <img alt="typescript" src="https://cdn.simpleicons.org/typescript" style="vertical-align: middle;" height="20" width="20" /> **TypeScript**, you can add panrpc to your project (both server-side TypeScript/Node.js and all major browser engines are supported) by running the following:

```shell
$ npm install @pojntfx/panrpc
```

### `purl` Tool

In addition to the library, the CLI tool `purl` is also available; `purl` is like [cURL](https://curl.se/) and [gRPCurl](https://github.com/fullstorydev/grpcurl), but for panrpc: A command-line tool for interacting with panrpc servers. `purl` is provided in the form of static binaries.

On Linux, you can install them like so:

```shell
$ curl -L -o /tmp/purl "https://github.com/pojntfx/panrpc/releases/latest/download/purl.linux-$(uname -m)"
$ sudo install /tmp/purl /usr/local/bin
```

On macOS, you can use the following:

```shell
$ curl -L -o /tmp/purl "https://github.com/pojntfx/panrpc/releases/latest/download/purl.darwin-$(uname -m)"
$ sudo install /tmp/purl /usr/local/bin
```

On Windows, the following should work (using PowerShell as administrator):

```PowerShell
Invoke-WebRequest https://github.com/pojntfx/panrpc/releases/latest/download/purl.windows-x86_64.exe -OutFile \Windows\System32\purl.exe
```

You can find binaries for more operating systems and architectures on [GitHub releases](https://github.com/pojntfx/panrpc/releases).

## Tutorial

### <img alt="Go" src="https://cdn.simpleicons.org/go" style="vertical-align: middle;" height="20" width="20" /> Go

> Just looking for sample code? Check out the sources for the example [coffee machine server](./go/cmd/panrpc-example-websocket-coffee-server-cli/main.go) and [coffee machine client/remote control](./go/cmd/panrpc-example-websocket-coffee-client-cli/main.go).

#### 1. Choosing a Transport and Serializer

<details>
  <summary>Expand section</summary>

Start by creating a new Go module for the tutorial and installing `github.com/pojntfx/panrpc/go`:

```shell
$ mkdir -p panrpc-tutorial-go
$ cd panrpc-tutorial-go
$ go mod init panrpc-tutorial-go
$ go get github.com/pojntfx/panrpc/go/...@latest
```

The Go version of panrpc supports many transports. While common ones are TCP, WebSockets, UNIX sockets or WebRTC, anything that directly implements or can be adapted to a [`io.ReadWriter`](https://pkg.go.dev/io#ReadWriter) can be used with the panrpc [`LinkStream` API](https://pkg.go.dev/github.com/pojntfx/panrpc/go/pkg/rpc#Registry.LinkStream). If you want to use a message broker like Valkey/Redis or NATS as the transport, or need more control over the wire protocol, you can use the [`LinkMessage` API](https://pkg.go.dev/github.com/pojntfx/panrpc/go/pkg/rpc#Registry.LinkMessage) instead. For this tutorial, we'll be using WebSockets as the transport through the `github.com/coder/websocket` library, which you can install like so:

```shell
$ go get github.com/coder/websocket@latest
```

In addition to supporting many transports, the Go version of panrpc also supports different serializers. Common ones are JSON and CBOR, but similarly to transports anything that implements or can be adapted to a `io.ReadWriter` stream can be used. For this tutorial, we'll be using JSON as the serializer through the `encoding/json` Go standard library.

</details>

#### 2. Creating a Server

In this tutorial we'll be creating a simple coffee machine server that simulates brewing coffee, and can be controlled by using a remote control (the coffee machine client).

<details>
  <summary>Expand section</summary>

To start with implementing the coffee machine server, create a new file `cmd/coffee-machine/main.go` and define a basic struct with a `BrewCoffee` method. This method simulates brewing coffee by validating the coffee variant, checking if there is enough water available to brew the coffee, sleeping for five seconds, and returning the new water level to the remote control:

```go
// cmd/coffee-machine/main.go

package main

import (
	"context"
	"errors"
	"log"
	"slices"
	"time"
)

type coffeeMachine struct {
	supportedVariants []string
	waterLevel        int
}

func (s *coffeeMachine) BrewCoffee(
	ctx context.Context,
	variant string,
	size int,
) (int, error) {
	if !slices.Contains(s.supportedVariants, variant) {
		return 0, errors.New("unsupported variant")
	}

	if s.waterLevel-size < 0 {
		return 0, errors.New("not enough water")
	}

	log.Println("Brewing coffee variant", variant, "in size", size, "ml")

	time.Sleep(time.Second * 5)

	s.waterLevel -= size

	return s.waterLevel, nil
}
```

> The following limitations on which methods can be exposed as RPCs exist:
>
> - Methods must have `context.Context` as their first argument
> - Methods can not have variadic arguments
> - Methods must return either an error or a single value and an error
> - Methods must be public (private methods won't be callable as RPCs, but stay callable as regular methods)

To start turning the `BrewCoffee` method into an RPC, create an instance of the struct and pass it to a [panrpc Registry](https://pkg.go.dev/github.com/pojntfx/panrpc/go/pkg/rpc#Registry) like so:

```go
// cmd/coffee-machine/main.go

import "github.com/pojntfx/panrpc/go/pkg/rpc"

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	service := &coffeeMachine{
		supportedVariants: []string{"latte", "americano"},
		waterLevel:        1000,
	}

	var clients atomic.Int64
	registry := rpc.NewRegistry[struct{}, json.RawMessage](
		service,

		&rpc.RegistryHooks{
			OnClientConnect: func(remoteID string) {
				log.Printf("%v remote controls connected", clients.Add(1))
			},
			OnClientDisconnect: func(remoteID string) {
				log.Printf("%v remote controls connected", clients.Add(-1))
			},
		},
	)
}
```

Now that we have a registry that provides our coffee machine's RPCs, we can link it to our transport (WebSockets) and serializer of choice (JSON). This requires a bit of boilerplate to upgrade from HTTP to WebSockets, so feel free to copy-and-paste this, or take a look at the [examples](#examples) to check out how you can set up a different transport (TCP, WebSockets, UNIX sockets etc.) and serializer (JSON, CBOR etc.) instead:

<details>
  <summary>Expand boilerplate code snippet</summary>

```go
// cmd/coffee-machine/main.go

import (
	"encoding/json"
	"net"
	"net/http"

	"github.com/pojntfx/panrpc/go/pkg/rpc"
	"github.com/coder/websocket"
)

func main() {
  // ...

  // Create TCP listener
	lis, err := net.Listen("tcp", "127.0.0.1:1337")
	if err != nil {
		panic(err)
	}
	defer lis.Close()

	log.Println("Listening on", lis.Addr())

	// Create HTTP server from TCP listener
	if err := http.Serve(lis, http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				w.WriteHeader(http.StatusInternalServerError)

				log.Printf("Remote control disconnected with error: %v", err)
			}
		}()

		// Upgrade from HTTP to WebSockets
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

			// Set up the streaming JSON encoder and decoder
			encoder := json.NewEncoder(conn)
			decoder := json.NewDecoder(conn)

			go func() {
				if err := registry.LinkStream(
          r.Context(),

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
}
```

</details>

**Congratulations!** You've created your first panrpc server. You can start it from your terminal like so:

```shell
$ go run ./cmd/coffee-machine/main.go
```

You should now see the following in your terminal, which means that the server is available on `localhost:1337`:

```plaintext
Listening on localhost:1337
```

</details>

#### 3. Creating a Client

In order to interact with the coffee machine server, we'll now create the remote control (the coffee machine client), which will call the `BrewCoffee` RPC.

<details>
  <summary>Expand section</summary>

To start with implementing the remote control, create a new file `cmd/remote-control/main.go` and define a basic struct with a placeholder method that mirrors the `BrewCoffee` RPC:

```go
// cmd/remote-control/main.go

package main

import "context"

type coffeeMachine struct {
	BrewCoffee func(
		ctx context.Context,
		variant string,
		size int,
	) (int, error)
}
```

In order to make the `BrewCoffee` placeholder method do RPC calls, create an instance of the struct and pass it to a [panrpc Registry](https://pkg.go.dev/github.com/pojntfx/panrpc/go/pkg/rpc#Registry) like so:

```go
// cmd/remote-control/main.go

import "github.com/pojntfx/panrpc/go/pkg/rpc"

func main() {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	var clients atomic.Int64
	registry := rpc.NewRegistry[coffeeMachine, json.RawMessage](
		&struct{}{},

		&rpc.RegistryHooks{
			OnClientConnect: func(remoteID string) {
				log.Printf("%v coffee machines connected", clients.Add(1))
			},
			OnClientDisconnect: func(remoteID string) {
				log.Printf("%v coffee machines connected", clients.Add(-1))
			},
		},
	)
}
```

Now that we have a registry that turns the remote control's placeholder methods into RPC calls, we can link it to our transport (WebSockets) and serializer of choice (JSON). Once again, this requires a bit of boilerplate to connect to the WebSocket, so feel free to copy-and-paste this, or take a look at the [examples](#examples) to check out how you can set up a different transport (TCP, WebSockets, UNIX sockets etc.) and serializer (JSON, CBOR etc.) instead:

<details>
  <summary>Expand boilerplate code snippet</summary>

```go
// cmd/remote-control/main.go

import (
	"encoding/json"

	"github.com/pojntfx/panrpc/go/pkg/rpc"
	"github.com/coder/websocket"
)

func main() {
  // ...

 // Connect to WebSocket server
	c, _, err := websocket.Dial(ctx, "ws://127.0.0.1:1337", nil)
	if err != nil {
		panic(err)
	}

	conn := websocket.NetConn(ctx, c, websocket.MessageText)
	defer conn.Close()

	log.Println("Connected to localhost:1337")

	// Set up the streaming JSON encoder and decoder
	encoder := json.NewEncoder(conn)
	decoder := json.NewDecoder(conn)

	if err := registry.LinkStream(
    ctx,

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
```

</details>

**Cheers!** You've created your first panrpc client. You can start it from your terminal like so:

```shell
$ go run ./cmd/remote-control/main.go
```

You should now see the following in your terminal, which means that the client has connected to the panrpc server at `localhost:1337`:

```plaintext
Connected to localhost:1337
1 coffee machines connected
```

Similarly so, the coffee machine server should output the following:

```plaintext
1 remote controls connected
```

</details>

#### 4. Calling the Server's RPCs from the Client

The coffee machine and the client are now connected to each other, but we haven't added the ability to call the `BrewCoffee` RPC from the remote control just yet. To fix this, we'll create a simple TUI interface that will print a list of available coffee variants and sizes to the terminal, waits for the user to make their choice by entering a number, and then calls the `BrewCoffee` RPC with the correct arguments. After the coffee has been brewed, we'll print the new water level to the terminal.

<details>
  <summary>Expand section</summary>

To achieve this, we can call this RPC transparently from the remote control by accessing the connected coffee machine(s) with `registry.ForRemotes`, and we can handle errors by checking with `if err := ..., err != nil { ... }` just like if we were making a local function call:

```go
// cmd/remote-control/main.go

import (
	"bufio"
	"log"
	"os"
)

func main() {
  // ...

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

  // ...
}
```

> Note that by cancelling the `context.Context` that we pass in as the first argument to every RPC call, you can cancel an RPC call before it has returned, which is useful for [implementing things like timeouts](https://pkg.go.dev/context#WithTimeout). If you don't cancel this `context.Context` like we do in this example, the RPC call will simply block until it returns.

Now we can restart the remote control like so:

```shell
$ go run ./cmd/remote-control/main.go
```

After which you should see the following output:

```plaintext
Enter one of the following numbers followed by <ENTER> to brew a coffee:

- 1: Brew small Cafè Latte
- 2: Brew large Cafè Latte

- 3: Brew small Americano
- 4: Brew large Americano
1 coffee machines connected
Connected to localhost:1337
```

It is now possible to brew a coffee by pressing a number and <kbd>ENTER</kbd>. Once the RPC has been called, the coffee machine should print something like the following:

```plaintext
Brewing coffee variant latte in size 100 ml
```

And after the coffee has been brewed, the remote control should return the remaining water level like so:

```plaintext
Remaining water: 900 ml
```

**Enjoy your (virtual) coffee!** You've successfully called an RPC provided by a server from the client. Feel free to try out the other supported variants and sizes until there is no more water remaining.

</details>

#### 5. Calling the Client's RPCs from the Server

So far, we've enabled a remote control/client to call the `BrewCoffee` RPC on the coffee machine/server. This however means that if multiple remote controls are connected to one coffee machine, only the remote control that called the RPC is aware of coffee being brewed. In order to notify the other remote controls that coffee is being brewed, we will use panrpc to call a new RPC on the remote control/client from the coffee machine/server each time we brew coffee.

<details>
  <summary>Expand section</summary>

To get started, we can once again create a basic struct on the client with a method `SetCoffeeMachineBrewing`, which will print the state of the coffee machine to the remote control's terminal:

```go
// cmd/remote-control/main.go

type remoteControl struct{}

func (s *remoteControl) SetCoffeeMachineBrewing(ctx context.Context, brewing bool) error {
	if brewing {
		log.Println("Coffee machine is now brewing")
	} else {
		log.Println("Coffee machine has stopped brewing")
	}

	return nil
}
```

To start turning this new `SetCoffeeMachineBrewing` method into an RPC that server can call, create an instance of the struct and pass it to the client's registry like so:

```go
// cmd/remote-control/main.go

func main() {
  // ...

  registry := rpc.NewRegistry[coffeeMachine, json.RawMessage](
		&remoteControl{},

		&rpc.RegistryHooks{
			OnClientConnect: func(remoteID string) {
				log.Printf("%v coffee machines connected", clients.Add(1))
			},
			OnClientDisconnect: func(remoteID string) {
				log.Printf("%v coffee machines connected", clients.Add(-1))
			},
		},
	)

  // ...
}
```

The remote control/client now exposes the `SetCoffeeMachineBrewing` RPC, and we can start enabling the coffee machine/server to call it by defining a basic struct with a method that mirrors the RPC, just like we did before on the remote control for `BrewCoffee`:

```go
// cmd/coffee-machine/main.go

type remoteControl struct {
	SetCoffeeMachineBrewing func(ctx context.Context, brewing bool) error
}
```

In order to make the `SetCoffeeMachineBrewing` placeholder method do RPC calls, create an instance of the struct and pass it to the server's registry like so:

```go
// cmd/coffee-machine/main.go

func main() {
  // ...

	registry := rpc.NewRegistry[remoteControl, json.RawMessage](
		service,

		&rpc.RegistryHooks{
			OnClientConnect: func(remoteID string) {
				log.Printf("%v remote controls connected", clients.Add(1))
			},
			OnClientDisconnect: func(remoteID string) {
				log.Printf("%v remote controls connected", clients.Add(-1))
			},
		},
	)

  // ...
}
```

The coffee machine/server and the remote control/client now both know of the new `SetCoffeeMachineBrewing` RPC, but the server doesn't call it yet. To fix this, we can call this RPC transparently from the coffee machine by accessing the connected remote control(s) with `registry.ForRemotes` just like we did before in the remote control, and we can handle errors by checking with `if err := ..., err != nil { ... }` just like if we were making a local function call. We'll also use the first argument to the RPC, `ctx`, in conjunction with `rpc.GetRemoteID` to get the ID of the remote control/client that is calling `BrewCoffee`, so that we don't call `SetCoffeeMachineBrewing` on the remote control/client that is calling `BrewCoffee` itself:

```go
// cmd/remote-control/main.go

type coffeeMachine struct {
	supportedVariants []string
	waterLevel        int

	ForRemotes func(
		cb func(remoteID string, remote remoteControl) error,
	) error
}

func (s *coffeeMachine) BrewCoffee(
	ctx context.Context,
	variant string,
	size int,
) (int, error) {
  // Get the ID of the remote control that's calling `BrewCoffee`
	targetID := rpc.GetRemoteID(ctx)

  // Notify connected remote controls that coffee is no longer brewing
	defer s.ForRemotes(func(remoteID string, remote remoteControl) error {
    // Don't call `SetCoffeeMachineBrewing` if it's the remote control that's calling `BrewCoffee`
		if remoteID == targetID {
			return nil
		}

		return remote.SetCoffeeMachineBrewing(ctx, false)
	})

  // Notify connected remote controls that coffee is brewing
	if err := s.ForRemotes(func(remoteID string, remote remoteControl) error {
    // Don't call `SetCoffeeMachineBrewing` if it's the remote control that's calling `BrewCoffee`
		if remoteID == targetID {
			return nil
		}

		return remote.SetCoffeeMachineBrewing(ctx, true)
	}); err != nil {
		return 0, err
	}

	if !slices.Contains(s.supportedVariants, variant) {
		return 0, errors.New("unsupported variant")
	}

	if s.waterLevel-size < 0 {
		return 0, errors.New("not enough water")
	}

	log.Println("Brewing coffee variant", variant, "in size", size, "ml")

	time.Sleep(time.Second * 5)

	s.waterLevel -= size

	return s.waterLevel, nil
}
```

Note that we've added the `forRemotes` field to the coffee machine/server; we can get the implementation for it from the registry like so:

```go
// cmd/coffee-machine/main.go

func main{
  service := // ...

  registry := // ...
  service.forRemotes = registry.ForRemotes;
}
```

Now that we've added support for this RPC to the coffee machine/server, we can restart it like so:

```shell
$ go run ./cmd/coffee-machine/main.go
```

To test if it works, connect two remote controls/clients to it like so:

```shell
$ go run ./cmd/remote-control/main.go
# In another terminal
$ go run ./cmd/remote-control/main.go
```

You can now request the coffee machine to brew a coffee on either of the remote controls by pressing a number and <kbd>ENTER</kbd>. Once the RPC has been called, the coffee machine should print something like the following again:

```plaintext
Brewing coffee variant latte in size 100 ml
```

And after the coffee has been brewed, the remote control that you've chosen to brew the coffee with should once again return the remaining water level like so:

```plaintext
Remaining water: 900 ml
```

The other connected remote controls will be notified that the coffee machine is brewing, and then once it has finished brewing:

```plaintext
Coffee machine is now brewing
Coffee machine has stopped brewing
```

**Enjoy your distributed coffee machine!** You've successfully called an RPC provided by a client from the server to implement multicast notifications, something that usually is quite complex to do with RPC systems.

</details>

#### 6. Passing Closures to RPCs

So far, when the remote control/client calls the `BrewCoffee` RPC, there is no way of knowing the incremental progress of the brew other than waiting for `BrewCoffee` to return the new water level. In order to know of the progress of the coffee machine as it is brewing, we can make use of the closure/callback support in panrpc, which allows us to pass a function to an RPC call, just like you could do locally.

<details>
  <summary>Expand section</summary>

First, we'll add a `onProgress` callback to the coffee machine's `BrewCoffee` implementation, which we then call incrementally during the brewing process:

```go
// cmd/coffee-machine/main.go

func (s *coffeeMachine) BrewCoffee(
	ctx context.Context,
	variant string,
	size int,
	onProgress func(ctx context.Context, percentage int) error, // This is new
) (int, error) {
	// ...

	// Report 0% brewing process
	if err := onProgress(ctx, 0); err != nil {
		return 0, err
	}

	// Report 25% brewing process
	time.Sleep(500 * time.Millisecond)
	if err := onProgress(ctx, 25); err != nil {
		return 0, err
	}

	// Report 50% brewing process
	time.Sleep(500 * time.Millisecond)
	if err := onProgress(ctx, 50); err != nil {
		return 0, err
	}

	// Report 75% brewing process
	time.Sleep(500 * time.Millisecond)
	if err := onProgress(ctx, 75); err != nil {
		return 0, err
	}

	// Report 100% brewing process
	time.Sleep(500 * time.Millisecond)
	if err := onProgress(ctx, 100); err != nil {
		return 0, err
	}

	// ..

	return s.waterLevel, nil
}
```

In the remote control, we'll also extend the struct with the `BrewCoffee` placeholder method with this new RPC argument:

```go
// cmd/remote-control/main.go

type coffeeMachine struct {
	BrewCoffee func(
		ctx context.Context,
		variant string,
		size int,
		onProgress func(ctx context.Context, percentage int) error, // This is new
	) (int, error)
}
```

And finally, where we call the `BrewCoffee` RPC in the remote control/client, we can pass in the implementation of this closure:

```go
// cmd/remote-control/main.go

go func() {
	// ...
	for {
		// ...
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
						log.Printf(`Brewing Cafè Latte ... %v%% done`, percentage) // This is new

						return nil
					},
				)

				// ...

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
						log.Printf(`Brewing Americano ... %v%% done`, percentage) // This is new

						return nil
					},
				)

				// ...

			return nil
		}); err != nil {
			panic(err)
		}
	}
}()
```

Now we can restart the coffee machine/server again like so:

```shell
$ go run ./cmd/coffee-machine/main.go
```

And connect the remote control/client to it again like so:

```shell
$ go run ./cmd/remote-control/main.go
```

You can now request the coffee machine to brew a coffee by pressing a number and <kbd>ENTER</kbd>. Once the RPC has been called, the coffee machine should print something like the following again:

```plaintext
Brewing coffee variant latte in size 100 ml
```

And the remote control will print the progress as reported by the coffee machine to the terminal, before once again returning the remaining water level like so:

```plaintext
Brewing Cafè Latte ... 0% done
Brewing Cafè Latte ... 25% done
Brewing Cafè Latte ... 50% done
Brewing Cafè Latte ... 75% done
Brewing Cafè Latte ... 100% done
Remaining water: 900 ml
```

**Enjoy your live coffee brewing progress!** You've successfully implemented incremental coffee brewing progress reports by using panrpc's closure support, something that is usually quite tricky to do with RPC frameworks.

</details>

#### 7. Nesting RPCs

So far, we've added RPCs directly to our coffee machine/server and remote control/client. While this approach is simple, it makes future extensions difficult. If we want to add more features, we would need to modify the coffee machine/server and remote control/client directly by adding new RPC methods, which can be hard to do in a type-safe way. Additionally, having only one level of RPCs makes large APIs hard to understand and organize as the number of RPCs increases.

In order to work around this, panrpc supports nesting RPCs in both clients and servers. This allows you to simplify top-level RPC calls; for example, instead of a single RPC like `GetConnectedChatUsers()`, you can use categorized, nested calls such as `Chat.Users.GetConnected()`.

<details>
  <summary>Expand section</summary>

To define a nested RPC, simply add another struct as a public property to your existing coffee machine/server or remote control/client. In this new stuct, you can define RPCs as methods, just like with top-level RPCs. To call them, follow the same concept: Define placeholder methods in the new struct and add it as a public property to your coffee machine/server or remote control/client.

In this example, we'll add a tea brewer extension to our coffee machine/server and remote control/client. This extension will allow us to list available tea variants by calling the `Extension.GetVariants` RPC. For brevity, we won't implement all the tea brewing functions. To add the tea brewer extension to the coffee machine/server, first create a new `TeaBrewer` struct, similar to the `CoffeeMachine` struct. Then, add the `GetVariants` method to this class and instantiate it with the supported variants:

```go
// cmd/coffee-machine/main.go

type teaBrewer struct {
	supportedVariants []string
}

func (s *teaBrewer) GetVariants(ctx context.Context) ([]string, error) {
	return s.supportedVariants, nil
}
```

To add these nested RPCs to the main `CoffeeMachine` struct, we'll include them as a public property in the struct. To keep the coffee machine flexible and avoid depending on the `TeaBrewer` extension, we'll name the property `Extension` and make it generic in `CoffeeMachine`. This way, we can easily replace the tea brewer with another extension in the future, such as a full-featured tea brewer:

```go
// cmd/coffee-machine/main.go

type coffeeMachine[E any] struct {
	Extension E

  // ...
}
```

Finally, for the coffee machine/server, create an instance of our extension and pass it to the `CoffeeMachine` when initializing it. This is also where you provide the list of supported tea variants:

```go
// cmd/coffee-machine/main.go

// ...
service := &coffeeMachine[*teaBrewer]{
		Extension: &teaBrewer{
			supportedVariants: []string{"darjeeling", "chai", "earlgrey"},
		},

		supportedVariants: []string{"latte", "americano"},
		waterLevel:        1000,
	}
// ...
```

For the remote control/client, it's a very similar process. First, we define the new `TeaBrewer` struct with the `GetVariants` placeholder method:

```go
// cmd/remote-control/main.go

type teaBrewer struct {
	GetVariants func(ctx context.Context) ([]string, error)
}
```

Then we'll add the nested RPCs to the main `CoffeeMachine` struct as a public instance property, and we'll use generics again to keep everything extensible:

```go
// cmd/remote-control/main.go

type coffeeMachine[E any] struct {
	Extension E

	// ...
}
```

After this, we need to create an instance of our extension and pass it to `CoffeeMachine` when we create it:

```go
// cmd/remote-control/main.go

registry := rpc.NewRegistry[coffeeMachine[teaBrewer], json.RawMessage](
		&remoteControl{},

		// ...
	)
```

And finally, we add another switch case to the remote control/client so that we can call `Extension.GetVariants`:

```go
// cmd/remote-control/main.go

	go func() {
		log.Println(`Enter one of the following numbers followed by <ENTER> to brew a coffee:

- 1: Brew small Cafè Latte
- 2: Brew large Cafè Latte

- 3: Brew small Americano
- 4: Brew large Americano

Or enter 5 to list available tea variants.`)

  // ...

  if err := registry.ForRemotes(func(remoteID string, remote coffeeMachine[teaBrewer]) error {
		switch line {
      // ..
      case "5\n":
				res, err := remote.Extension.GetVariants(ctx)
				if err != nil {
					log.Println("Couldn't list available tea variants:", err)

					return nil
				}

				log.Println("Available tea variants:", res)

      default:
      // ..
		}
	}
}()
```

Now we can restart the coffee machine/server again like so:

```shell
$ go run ./cmd/coffee-machine/main.go
```

And connect the remote control/client to it again like so:

```shell
$ go run ./cmd/remote-control/main.go
```

You can now request the coffee machine to list the available tea variants by pressing `5` and <kbd>ENTER</kbd>. Once the RPC has been called, the remote control should print something like the following:

```plaintext
Available tea variants: [ "darjeeling", "chai", "earlgrey" ]
```

**🚀 That's it!** You've successfully built a virtual coffee machine with support for brewing coffee, notifications when coffee is being brewed, and incremental coffee brewing progress reports. You've also made it easily extensible by using nested RPCs. We can't wait to see what you're going to build next with panrpc! Be sure to take a look at the [reference](#reference) and [examples](#examples) for more information, or check out the complete sources for the [coffee machine server](./go/cmd/panrpc-example-websocket-coffee-server-cli/main.go) and [coffee machine client/remote control](./go/cmd/panrpc-example-websocket-coffee-client-cli/main.go) for a recap.

</details>

### <img alt="typescript" src="https://cdn.simpleicons.org/typescript" style="vertical-align: middle;" height="20" width="20" /> TypeScript

> Just looking for sample code? Check out the sources for the example [coffee machine server](./ts/bin/panrpc-example-websocket-coffee-server-cli.ts) and [coffee machine client/remote control](./ts/bin/panrpc-example-websocket-coffee-client-cli.ts).

#### 1. Choosing a Transport and Serializer

<details>
  <summary>Expand section</summary>

Start by creating a new npm module for the tutorial and installing `@pojntfx/panrpc`:

```shell
$ mkdir -p panrpc-tutorial-typescript
$ cd panrpc-tutorial-typescript
$ npm init -y
$ npm install @pojntfx/panrpc
```

The TypeScript version of panrpc supports many transports. While common ones are TCP, WebSockets, UNIX sockets or WebRTC, anything that directly implements or can be adapted to a [WHATWG stream](https://developer.mozilla.org/en-US/docs/Web/API/Streams_API) can be used with the panrpc [`linkStream` API](https://pojntfx.github.io/panrpc/classes/Registry.html#linkStream). If you want to use a message broker like Valkey/Redis or NATS as the transport, or need more control over the wire protocol, you can use the [`linkMessage` API](https://pojntfx.github.io/panrpc/classes/Registry.html#linkMessage) instead. For this tutorial, we'll be using WebSockets as the transport through the `ws` library, which you can install like so:

```shell
$ npm install ws
```

In addition to supporting many transports, the TypeScript version of panrpc also supports different serializers. Common ones are JSON and CBOR, but similarly to transports anything that implements or can be adapted to a WHATWG stream can be used. For this tutorial, we'll be using JSON as the serializer through the `@streamparser/json-whatwg` library, which you can install like so:

```shell
$ npm install @streamparser/json-whatwg
```

</details>

#### 2. Creating a Server

In this tutorial we'll be creating a simple coffee machine server that simulates brewing coffee, and can be controlled by using a remote control (the coffee machine client).

<details>
  <summary>Expand section</summary>

To start with implementing the coffee machine server, create a new file `coffee-machine.ts` and define a basic class with a `BrewCoffee` method. This method simulates brewing coffee by validating the coffee variant, checking if there is enough water available to brew the coffee, sleeping for five seconds, and returning the new water level to the remote control:

```typescript
// coffee-machine.ts

import { ILocalContext } from "@pojntfx/panrpc";

class CoffeeMachine {
  #waterLevel: number;

  constructor(private supportedVariants: string[], waterLevel: number) {
    this.#waterLevel = waterLevel;

    this.BrewCoffee = this.BrewCoffee.bind(this);
  }

  async BrewCoffee(
    ctx: ILocalContext,
    variant: string,
    size: number
  ): Promise<number> {
    if (!this.supportedVariants.includes(variant)) {
      throw new Error("unsupported variant");
    }

    if (this.#waterLevel - size < 0) {
      throw new Error("not enough water");
    }

    console.log("Brewing coffee variant", variant, "in size", size, "ml");

    await new Promise((r) => {
      setTimeout(r, 5000);
    });

    this.#waterLevel -= size;

    return this.#waterLevel;
  }
}
```

> The following limitations on which methods can be exposed as RPCs exist:
>
> - Methods must have `ILocalContext` as their first argument
> - Methods can not have variadic arguments
> - Methods must be public (private methods - those that start with `#`, see [private properties](https://developer.mozilla.org/en-US/docs/Web/JavaScript/Reference/Classes/Private_properties) - won't be callable as RPCs, but stay callable as regular methods)

To start turning the `BrewCoffee` method into an RPC, create an instance of the class and pass it to a [panrpc Registry](https://pojntfx.github.io/panrpc/classes/Registry.html) like so:

```typescript
// coffee-machine.ts

import { Registry } from "@pojntfx/panrpc";

const service = new CoffeeMachine(["latte", "americano"], 1000);

let clients = 0;

const registry = new Registry(
  service,
  new (class {})(),

  {
    onClientConnect: () => {
      clients++;

      console.log(clients, "remote controls connected");
    },
    onClientDisconnect: () => {
      clients--;

      console.log(clients, "remote controls connected");
    },
  }
);
```

Now that we have a registry that provides our coffee machine's RPCs, we can link it to our transport (WebSockets) and serializer of choice (JSON). This requires a bit of boilerplate since the `ws` library doesn't provide WHATWG streams directly yet, so feel free to copy-and-paste this, or take a look at the [examples](#examples) to check out how you can set up a different transport (TCP, WebSockets, UNIX sockets etc.) and serializer (JSON, CBOR etc.) instead:

<details>
  <summary>Expand boilerplate code snippet</summary>

```typescript
// coffee-machine.ts

import { JSONParser } from "@streamparser/json-whatwg";
import { WebSocketServer } from "ws";

// Create WebSocket server
const server = new WebSocketServer({
  host: "127.0.0.1",
  port: 1337,
});

server.on("connection", (socket) => {
  socket.addEventListener("error", (e) => {
    console.error("Remote control disconnected with error:", e);
  });

  const linkSignal = new AbortController();

  // Set up streaming JSON encoder
  const encoder = new WritableStream({
    write(chunk) {
      socket.send(JSON.stringify(chunk));
    },
  });

  // Set up streaming JSON decoder
  const parser = new JSONParser({
    paths: ["$"],
    separator: "",
  });
  const parserWriter = parser.writable.getWriter();
  const parserReader = parser.readable.getReader();
  const decoder = new ReadableStream({
    start(controller) {
      parserReader
        .read()
        .then(async function process({ done, value }) {
          if (done) {
            controller.close();

            return;
          }

          controller.enqueue(value?.value);

          parserReader
            .read()
            .then(process)
            .catch((e) => controller.error(e));
        })
        .catch((e) => controller.error(e));
    },
  });
  socket.addEventListener("message", (m) =>
    parserWriter.write(m.data as string)
  );
  socket.addEventListener("close", () => {
    parserReader.cancel();
    parserWriter.abort();
    linkSignal.abort();
  });

  registry.linkStream(
    linkSignal.signal,

    encoder,
    decoder,

    (v) => v,
    (v) => v
  );
});

console.log("Listening on localhost:1337");
```

</details>

**Congratulations!** You've created your first panrpc server. You can start it from your terminal like so:

```shell
$ npx tsx coffee-machine.ts
```

You should now see the following in your terminal, which means that the server is available on `localhost:1337`:

```plaintext
Listening on localhost:1337
```

</details>

#### 3. Creating a Client

In order to interact with the coffee machine server, we'll now create the remote control (the coffee machine client), which will call the `BrewCoffee` RPC.

<details>
  <summary>Expand section</summary>

To start with implementing the remote control, create a new file `remote-control.ts` and define a basic class with a placeholder method that mirrors the `BrewCoffee` RPC:

```typescript
// remote-control.ts

import { IRemoteContext } from "@pojntfx/panrpc";

class CoffeeMachine {
  async BrewCoffee(
    ctx: IRemoteContext,
    variant: string,
    size: number
  ): Promise<number> {
    return 0;
  }
}
```

> Placeholder methods must have `IRemoteContext` instead of `ILocalContext` as their first argument.

In order to make the `BrewCoffee` placeholder method do RPC calls, create an instance of the class and pass it to a [panrpc Registry](https://pojntfx.github.io/panrpc/classes/Registry.html) like so:

```typescript
// remote-control.ts

import { Registry } from "@pojntfx/panrpc";

let clients = 0;

const registry = new Registry(
  new (class {})(),
  new CoffeeMachine(),

  {
    onClientConnect: () => {
      clients++;

      console.log(clients, "coffee machines connected");
    },
    onClientDisconnect: () => {
      clients--;

      console.log(clients, "coffee machines connected");
    },
  }
);
```

Now that we have a registry that turns the remote control's placeholder methods into RPC calls, we can link it to our transport (WebSockets) and serializer of choice (JSON). Once again, this requires a bit of boilerplate since the `ws` library doesn't provide WHATWG streams directly yet, so feel free to copy-and-paste this, or take a look at the [examples](#examples) to check out how you can set up a different transport (TCP, WebSockets, UNIX sockets etc.) and serializer (JSON, CBOR etc.) instead:

<details>
  <summary>Expand boilerplate code snippet</summary>

```typescript
// remote-control.ts

import { JSONParser } from "@streamparser/json-whatwg";
import { WebSocket } from "ws";

// Connect to WebSocket server
const socket = new WebSocket("ws://127.0.0.1:1337");

socket.addEventListener("error", (e) => {
  console.error("Disconnected with error:", e);

  exit(1);
});
socket.addEventListener("close", () => exit(0));

await new Promise<void>((res, rej) => {
  socket.addEventListener("open", () => res());
  socket.addEventListener("error", rej);
});

const linkSignal = new AbortController();

// Set up streaming JSON encoder
const encoder = new WritableStream({
  write(chunk) {
    socket.send(JSON.stringify(chunk));
  },
});

// Set up streaming JSON decoder
const parser = new JSONParser({
  paths: ["$"],
  separator: "",
});
const parserWriter = parser.writable.getWriter();
const parserReader = parser.readable.getReader();
const decoder = new ReadableStream({
  start(controller) {
    parserReader
      .read()
      .then(async function process({ done, value }) {
        if (done) {
          controller.close();

          return;
        }

        controller.enqueue(value?.value);

        parserReader
          .read()
          .then(process)
          .catch((e) => controller.error(e));
      })
      .catch((e) => controller.error(e));
  },
});
socket.addEventListener("message", (m) => parserWriter.write(m.data as string));
socket.addEventListener("close", () => {
  parserReader.cancel();
  parserWriter.abort();
  linkSignal.abort();
});

registry.linkStream(
  linkSignal.signal,

  encoder,
  decoder,

  (v) => v,
  (v) => v
);

console.log("Connected to localhost:1337");
```

</details>

**Cheers!** You've created your first panrpc client. You can start it from your terminal like so:

```shell
$ npx tsx remote-control.ts
```

You should now see the following in your terminal, which means that the client has connected to the panrpc server at `localhost:1337`:

```plaintext
Connected to localhost:1337
1 coffee machines connected
```

Similarly so, the coffee machine server should output the following:

```plaintext
1 remote controls connected
```

</details>

#### 4. Calling the Server's RPCs from the Client

The coffee machine and the client are now connected to each other, but we haven't added the ability to call the `BrewCoffee` RPC from the remote control just yet. To fix this, we'll create a simple TUI interface that will print a list of available coffee variants and sizes to the terminal, waits for the user to make their choice by entering a number, and then calls the `BrewCoffee` RPC with the correct arguments. After the coffee has been brewed, we'll print the new water level to the terminal.

<details>
  <summary>Expand section</summary>

To achieve this, we can call this RPC transparently from the remote control by accessing the connected coffee machine(s) with `registry.forRemotes`, and we can handle errors with `try catch` just like if we were making a local function call:

```typescript
// remote-control.ts

import { createInterface } from "readline/promises";

(async () => {
  console.log(`Enter one of the following numbers followed by <ENTER> to brew a coffee:

- 1: Brew small Cafè Latte
- 2: Brew large Cafè Latte

- 3: Brew small Americano
- 4: Brew large Americano`);

  const rl = createInterface({ input: stdin, output: stdout });

  while (true) {
    const line = await rl.question("");

    await registry.forRemotes(async (remoteID, remote) => {
      switch (line) {
        case "1":
        case "2":
          try {
            const res = await remote.BrewCoffee(
              undefined,
              "latte",
              line === "1" ? 100 : 200
            );

            console.log("Remaining water:", res, "ml");
          } catch (e) {
            console.error(`Couldn't brew Cafè Latte: ${e}`);
          }

          break;

        case "3":
        case "4":
          try {
            const res = await remote.BrewCoffee(
              undefined,
              "americano",
              line === "3" ? 100 : 200
            );

            console.log("Remaining water:", res, "ml");
          } catch (e) {
            console.error(`Couldn't brew Americano: ${e}`);
          }

          break;

        default:
          console.log(`Unknown letter ${line}, ignoring input`);
      }
    });
  }
})();
```

> Note that by aborting the [AbortSignal](https://developer.mozilla.org/en-US/docs/Web/API/AbortSignal) that we can pass in as the first argument to every RPC call, you can abort an RPC call before it has returned, which is useful for [implementing things like timeouts](https://developer.mozilla.org/en-US/docs/Web/API/AbortSignal#aborting_a_fetch_operation_with_a_timeout). If you don't abort this `AbortSignal`, or pass in `undefined` like we do in this example, the RPC call will simply block until it returns.

Now we can restart the remote control like so:

```shell
$ npx tsx remote-control.ts
```

After which you should see the following output:

```plaintext
Enter one of the following numbers followed by <ENTER> to brew a coffee:

- 1: Brew small Cafè Latte
- 2: Brew large Cafè Latte

- 3: Brew small Americano
- 4: Brew large Americano
1 coffee machines connected
Connected to localhost:1337
```

It is now possible to brew a coffee by pressing a number and <kbd>ENTER</kbd>. Once the RPC has been called, the coffee machine should print something like the following:

```plaintext
Brewing coffee variant latte in size 100 ml
```

And after the coffee has been brewed, the remote control should return the remaining water level like so:

```plaintext
Remaining water: 900 ml
```

**Enjoy your (virtual) coffee!** You've successfully called an RPC provided by a server from the client. Feel free to try out the other supported variants and sizes until there is no more water remaining.

</details>

#### 5. Calling the Client's RPCs from the Server

So far, we've enabled a remote control/client to call the `BrewCoffee` RPC on the coffee machine/server. This however means that if multiple remote controls are connected to one coffee machine, only the remote control that called the RPC is aware of coffee being brewed. In order to notify the other remote controls that coffee is being brewed, we will use panrpc to call a new RPC on the remote control/client from the coffee machine/server each time we brew coffee.

<details>
  <summary>Expand section</summary>

To get started, we can once again create a basic class on the client with a method `SetCoffeeMachineBrewing`, which will print the state of the coffee machine to the remote control's terminal:

```typescript
// remote-control.ts

class RemoteControl {
  async SetCoffeeMachineBrewing(ctx: ILocalContext, brewing: boolean) {
    if (brewing) {
      console.log("Coffee machine is now brewing");
    } else {
      console.log("Coffee machine has stopped brewing");
    }
  }
}
```

To start turning this new `SetCoffeeMachineBrewing` method into an RPC that server can call, create an instance of the class and pass it to the client's registry like so:

```typescript
// remote-control.ts

const registry = new Registry(
  new RemoteControl(), // This line is new
  new CoffeeMachine(),

  {
    onClientConnect: () => {
      clients++;

      console.log(clients, "coffee machines connected");
    },
    onClientDisconnect: () => {
      clients--;

      console.log(clients, "coffee machines connected");
    },
  }
);
```

The remote control/client now exposes the `SetCoffeeMachineBrewing` RPC, and we can start enabling the coffee machine/server to call it by defining a basic class with a method that mirrors the RPC, just like we did before on the remote control for `BrewCoffee`:

```typescript
// coffee-machine.ts

class RemoteControl {
  async SetCoffeeMachineBrewing(ctx: IRemoteContext, brewing: boolean) {}
}
```

In order to make the `SetCoffeeMachineBrewing` placeholder method do RPC calls, create an instance of the class and pass it to the server's registry like so:

```typescript
// coffee-machine.ts

const registry = new Registry(
  service,
  new RemoteControl(), // This line is new

  {
    onClientConnect: () => {
      clients++;

      console.log(clients, "remote controls connected");
    },
    onClientDisconnect: () => {
      clients--;

      console.log(clients, "remote controls connected");
    },
  }
);
```

The coffee machine/server and the remote control/client now both know of the new `SetCoffeeMachineBrewing` RPC, but the server doesn't call it yet. To fix this, we can call this RPC transparently from the coffee machine by accessing the connected remote control(s) with `registry.forRemotes` just like we did before in the remote control, and we can handle errors with `try catch` just like if we were making a local function call. We'll also use the first argument to the RPC, `ILocalContext`, to get the ID of the remote control/client that is calling `BrewCoffee`, so that we don't call `SetCoffeeMachineBrewing` on the remote control/client that is calling `BrewCoffee` itself:

```typescript
// coffee-machine.ts

class CoffeeMachine {
  public forRemotes?: (
    cb: (remoteID: string, remote: RemoteControl) => Promise<void>
  ) => Promise<void>;

  // ...

  async BrewCoffee(
    ctx: ILocalContext,
    variant: string,
    size: number
  ): Promise<number> {
    // Get the ID of the remote control that's calling `BrewCoffee`
    const { remoteID: targetID } = ctx;

    try {
      // Notify connected remote controls that coffee is brewing
      await this.forRemotes?.(async (remoteID, remote) => {
        // Don't call `SetCoffeeMachineBrewing` if it's the remote control that's calling `BrewCoffee`
        if (remoteID === targetID) {
          return;
        }

        await remote.SetCoffeeMachineBrewing(undefined, true);
      });

      if (!this.supportedVariants.includes(variant)) {
        throw new Error("unsupported variant");
      }

      if (this.#waterLevel - size < 0) {
        throw new Error("not enough water");
      }

      console.log("Brewing coffee variant", variant, "in size", size, "ml");

      await new Promise((r) => {
        setTimeout(r, 5000);
      });
    } finally {
      // Notify connected remote controls that coffee is no longer brewing
      await this.forRemotes?.(async (remoteID, remote) => {
        // Don't call `SetCoffeeMachineBrewing` if it's the remote control that's calling `BrewCoffee`
        if (remoteID === targetID) {
          return;
        }

        await remote.SetCoffeeMachineBrewing(undefined, false);
      });
    }

    this.#waterLevel -= size;

    return this.#waterLevel;
  }
}
```

Note that we've added the `forRemotes` field to the coffee machine/server; we can get the implementation for it from the registry like so:

```typescript
// coffee-machine.ts

const service = // ...

const registry = // ...

service.forRemotes = registry.forRemotes;
```

Now that we've added support for this RPC to the coffee machine/server, we can restart it like so:

```shell
$ npx tsx coffee-machine.ts
```

To test if it works, connect two remote controls/clients to it like so:

```shell
$ npx tsx remote-control.ts
# In another terminal
$ npx tsx remote-control.ts
```

You can now request the coffee machine to brew a coffee on either of the remote controls by pressing a number and <kbd>ENTER</kbd>. Once the RPC has been called, the coffee machine should print something like the following again:

```plaintext
Brewing coffee variant latte in size 100 ml
```

And after the coffee has been brewed, the remote control that you've chosen to brew the coffee with should once again return the remaining water level like so:

```plaintext
Remaining water: 900 ml
```

The other connected remote controls will be notified that the coffee machine is brewing, and then once it has finished brewing:

```plaintext
Coffee machine is now brewing
Coffee machine has stopped brewing
```

**Enjoy your distributed coffee machine!** You've successfully called an RPC provided by a client from the server to implement multicast notifications, something that usually is quite complex to do with RPC systems.

</details>

#### 6. Passing Closures to RPCs

So far, when the remote control/client calls the `BrewCoffee` RPC, there is no way of knowing the incremental progress of the brew other than waiting for `BrewCoffee` to return the new water level. In order to know of the progress of the coffee machine as it is brewing, we can make use of the closure/callback support in panrpc, which allows us to pass a function to an RPC call, just like you could do locally.

<details>
  <summary>Expand section</summary>

First, we'll add a `onProgress` callback to the coffee machine's `BrewCoffee` implementation and decorate it with [panrpc's @remoteClosure decorator](https://pojntfx.github.io/panrpc/functions/remoteClosure.html), which we then call incrementally during the brewing process:

```typescript
// coffee-machine.ts

import { remoteClosure } from "@pojntfx/panrpc";

class CoffeeMachine {
  // ...

  async BrewCoffee(
    ctx: ILocalContext,
    variant: string,
    size: number,
    @remoteClosure
    onProgress: (ctx: IRemoteContext, percentage: number) => Promise<void> // This is new
  ): Promise<number> {
    // ...

    try {
      // ...

      // Report 0% brewing process
      await onProgress(undefined, 0);

      // Report 25% brewing process
      await new Promise((r) => {
        setTimeout(r, 500);
      });
      await onProgress(undefined, 25);

      // Report 50% brewing process
      await new Promise((r) => {
        setTimeout(r, 500);
      });
      await onProgress(undefined, 50);

      // Report 75% brewing process
      await new Promise((r) => {
        setTimeout(r, 500);
      });
      await onProgress(undefined, 75);

      // Report 100% brewing process
      await new Promise((r) => {
        setTimeout(r, 500);
      });
      await onProgress(undefined, 100);
    }

    // ..

    return this.#waterLevel;
  }
}
```

In the remote control, we'll also extend the class with the `BrewCoffee` placeholder method with this new RPC argument:

```typescript
// remote-control.ts

class CoffeeMachine {
  async BrewCoffee(
    ctx: IRemoteContext,
    variant: string,
    size: number,
    onProgress: (ctx: ILocalContext, percentage: number) => Promise<void> // This is new
  ): Promise<number> {
    return 0;
  }
}
```

And finally, where we call the `BrewCoffee` RPC in the remote control/client, we can pass in the implementation of this closure:

```typescript
// remote-control.ts

(async () => {
  // ...
  await registry.forRemotes(async (remoteID, remote) => {
    switch (line) {
      case "1":
      case "2":
        // ...
        const res = await remote.BrewCoffee(
          undefined,
          "latte",
          line === "1" ? 100 : 200,
          async (ctx, percentage) =>
            console.log(`Brewing Cafè Latte ... ${percentage}% done`) // This is new
        );

      // ...

      case "3":
      case "4":
        // ...
        const res = await remote.BrewCoffee(
          undefined,
          "americano",
          line === "3" ? 100 : 200,
          async (ctx, percentage) =>
            console.log(`Brewing Americano ... ${percentage}% done`) // This is new
        );

      // ..
    }
  });
})();
```

Now we can restart the coffee machine/server again like so:

```shell
$ npx tsx coffee-machine.ts
```

And connect the remote control/client to it again like so:

```shell
$ npx tsx remote-control.ts
```

You can now request the coffee machine to brew a coffee by pressing a number and <kbd>ENTER</kbd>. Once the RPC has been called, the coffee machine should print something like the following again:

```plaintext
Brewing coffee variant latte in size 100 ml
```

And the remote control will print the progress as reported by the coffee machine to the terminal, before once again returning the remaining water level like so:

```plaintext
Brewing Cafè Latte ... 0% done
Brewing Cafè Latte ... 25% done
Brewing Cafè Latte ... 50% done
Brewing Cafè Latte ... 75% done
Brewing Cafè Latte ... 100% done
Remaining water: 900 ml
```

**Enjoy your live coffee brewing progress!** You've successfully implemented incremental coffee brewing progress reports by using panrpc's closure support, something that is usually quite tricky to do with RPC frameworks.

</details>

#### 7. Nesting RPCs

So far, we've added RPCs directly to our coffee machine/server and remote control/client. While this approach is simple, it makes future extensions difficult. If we want to add more features, we would need to modify the coffee machine/server and remote control/client directly by adding new RPC methods, which can be hard to do in a type-safe way. Additionally, having only one level of RPCs makes large APIs hard to understand and organize as the number of RPCs increases.

In order to work around this, panrpc supports nesting RPCs in both clients and servers. This allows you to simplify top-level RPC calls; for example, instead of a single RPC like `GetConnectedChatUsers()`, you can use categorized, nested calls such as `Chat.Users.GetConnected()`.

<details>
  <summary>Expand section</summary>

To define a nested RPC, simply add another class as a public instance property to your existing coffee machine/server or remote control/client. In this new class, you can define RPCs as methods, just like with top-level RPCs. To call them, follow the same concept: Define placeholder methods in the new class and add it as a public instance property to your coffee machine/server or remote control/client.

In this example, we'll add a tea brewer extension to our coffee machine/server and remote control/client. This extension will allow us to list available tea variants by calling the `Extension.GetVariants` RPC. For brevity, we won't implement all the tea brewing functions. To add the tea brewer extension to the coffee machine/server, first create a new `TeaBrewer` class, similar to the `CoffeeMachine` class. Then, add the `GetVariants` method to this class and instantiate it with the supported variants:

```typescript
// coffee-machine.ts

class TeaBrewer {
  #supportedVariants: string[];

  constructor(supportedVariants: string[]) {
    this.#supportedVariants = supportedVariants;

    this.GetVariants = this.GetVariants.bind(this);
  }

  async GetVariants(ctx: ILocalContext): Promise<string[]> {
    return this.#supportedVariants;
  }
}
```

To add these nested RPCs to the main `CoffeeMachine` class, we'll include them as a public property in the class’s constructor. To keep the coffee machine flexible and avoid depending on the `TeaBrewer` extension, we'll name the property `Extension` and make it generic in `CoffeeMachine`. This way, we can easily replace the tea brewer with another extension in the future, such as a full-featured tea brewer:

```typescript
// coffee-machine.ts

class CoffeeMachine<E> {
  // ...
  constructor(
    public Extension: E,
    supportedVariants: string[],
    waterLevel: number
  ) {
    this.#supportedVariants = supportedVariants;
    this.#waterLevel = waterLevel;

    this.BrewCoffee = this.BrewCoffee.bind(this);
  }
  // ...
}
```

Finally, for the coffee machine/server, create an instance of our extension and pass it to the `CoffeeMachine` when initializing it. This is also where you provide the list of supported tea variants:

```typescript
// coffee-machine.ts

// ...
const service = new CoffeeMachine(
  new TeaBrewer(["darjeeling", "chai", "earlgrey"]),
  ["latte", "americano"],
  1000
);
// ...
```

For the remote control/client, it's a very similar process. First, we define the new `TeaBrewer` class with the `GetVariants` placeholder method:

```typescript
// remote-control.ts

class TeaBrewer {
  async GetVariants(ctx: IRemoteContext): Promise<string[]> {
    return [];
  }
}
```

Then we'll add the nested RPCs to the main `CoffeeMachine` class as a public instance property, and we'll use generics again to keep everything extensible:

```typescript
// remote-control.ts

class CoffeeMachine<E> {
  constructor(public Extension: E) {}

  // ...
}
```

After this, we need to create an instance of our extension and pass it to `CoffeeMachine` when we create it:

```typescript
// remote-control.ts

const registry = new Registry(
  new RemoteControl(),
  new CoffeeMachine(new TeaBrewer())

  // ...
);
```

And finally, we add another switch case to the remote control/client so that we can call `Extension.GetVariants`:

```typescript
// remote-control.ts

(async () => {
  console.log(`Enter one of the following numbers followed by <ENTER> to brew a coffee:

- 1: Brew small Cafè Latte
- 2: Brew large Cafè Latte

- 3: Brew small Americano
- 4: Brew large Americano

Or enter 5 to list available tea variants.`);

  // ...

  await registry.forRemotes(async (remoteID, remote) => {
    switch (line) {
      // ..
      case "5":
        try {
          const res = await remote.Extension.GetVariants(undefined);

          console.log("Available tea variants:", res);
        } catch (e) {
          console.error(`Couldn't list available tea variants: ${e}`);
        }

        break;

      default:
      // ..
    }
  });
})();
```

Now we can restart the coffee machine/server again like so:

```shell
$ npx tsx coffee-machine.ts
```

And connect the remote control/client to it again like so:

```shell
$ npx tsx remote-control.ts
```

You can now request the coffee machine to list the available tea variants by pressing `5` and <kbd>ENTER</kbd>. Once the RPC has been called, the remote control should print something like the following:

```plaintext
Available tea variants: [ "darjeeling", "chai", "earlgrey" ]
```

**🚀 That's it!** You've successfully built a virtual coffee machine with support for brewing coffee, notifications when coffee is being brewed, and incremental coffee brewing progress reports. You've also made it easily extensible by using nested RPCs. We can't wait to see what you're going to build next with panrpc! Be sure to take a look at the [reference](#reference) and [examples](#examples) for more information, or check out the complete sources for the [coffee machine server](./ts/bin/panrpc-example-websocket-coffee-server-cli.ts) and [coffee machine client/remote control](./ts/bin/panrpc-example-websocket-coffee-client-cli.ts) for a recap.

</details>

## Reference

### Library API

- [![Go Reference](https://pkg.go.dev/badge/github.com/pojntfx/panrpc/go.svg)](https://pkg.go.dev/github.com/pojntfx/panrpc/go)
- [![TypeScript docs](https://img.shields.io/badge/TypeScript%20-docs-blue.svg)](https://pojntfx.github.io/panrpc)

### Examples

To make getting started with panrpc easier, take a look at the following examples:

- **Transports**
  - **TCP (Stream-Oriented API)**
    - [<img alt="Go" src="https://cdn.simpleicons.org/go" style="vertical-align: middle;" height="20" width="20" /> TCP Server CLI Example](./go/cmd/panrpc-example-tcp-server-cli/main.go)
    - [<img alt="Go" src="https://cdn.simpleicons.org/go" style="vertical-align: middle;" height="20" width="20" /> TCP Server CLI Example (with graceful shutdown)](./go/cmd/panrpc-example-tcp-server-graceful-cli/main.go)
    - [<img alt="Go" src="https://cdn.simpleicons.org/go" style="vertical-align: middle;" height="20" width="20" /> TCP Client CLI Example](./go/cmd/panrpc-example-tcp-client-cli/main.go)
    - [<img alt="Go" src="https://cdn.simpleicons.org/go" style="vertical-align: middle;" height="20" width="20" /> TCP Client CLI Example (with graceful shutdown)](./go/cmd/panrpc-example-tcp-client-graceful-cli/main.go)
    - [<img alt="typescript" src="https://cdn.simpleicons.org/typescript" style="vertical-align: middle;" height="20" width="20" /> TCP Server CLI Example](./ts/bin/panrpc-example-tcp-server-cli.ts)
    - [<img alt="typescript" src="https://cdn.simpleicons.org/typescript" style="vertical-align: middle;" height="20" width="20" /> TCP Client CLI Example](./ts/bin/panrpc-example-tcp-client-cli.ts)
  - **UNIX Socket (Stream-Oriented API)**
    - [<img alt="Go" src="https://cdn.simpleicons.org/go" style="vertical-align: middle;" height="20" width="20" /> UNIX Socket Server CLI Example](./go/cmd/panrpc-example-unix-server-cli/main.go)
    - [<img alt="Go" src="https://cdn.simpleicons.org/go" style="vertical-align: middle;" height="20" width="20" /> UNIX Socket Client CLI Example](./go/cmd/panrpc-example-unix-client-cli/main.go)
    - [<img alt="typescript" src="https://cdn.simpleicons.org/typescript" style="vertical-align: middle;" height="20" width="20" /> UNIX Socket Server CLI Example](./ts/bin/panrpc-example-unix-server-cli.ts)
    - [<img alt="typescript" src="https://cdn.simpleicons.org/typescript" style="vertical-align: middle;" height="20" width="20" /> UNIX Socket Client CLI Example](./ts/bin/panrpc-example-unix-client-cli.ts)
  - **`stdin/stdout` Pipe (Stream-Oriented API)**
    - [<img alt="Go" src="https://cdn.simpleicons.org/go" style="vertical-align: middle;" height="20" width="20" /> `stdin/stdout` Pipe Socket Server CLI Example](./go/cmd/panrpc-example-pipe-server-cli/main.go)
    - [<img alt="Go" src="https://cdn.simpleicons.org/go" style="vertical-align: middle;" height="20" width="20" /> `stdin/stdout` Pipe Socket Client CLI Example](./go/cmd/panrpc-example-pipe-client-cli/main.go)
    - [<img alt="typescript" src="https://cdn.simpleicons.org/typescript" style="vertical-align: middle;" height="20" width="20" /> `stdin/stdout` Pipe Server CLI Example](./ts/bin/panrpc-example-pipe-server-cli.ts)
    - [<img alt="typescript" src="https://cdn.simpleicons.org/typescript" style="vertical-align: middle;" height="20" width="20" /> `stdin/stdout` Pipe Client CLI Example](./ts/bin/panrpc-example-pipe-client-cli.ts)
  - **WebSocket (Stream-Oriented API)**
    - <img alt="Go" src="https://cdn.simpleicons.org/go" style="vertical-align: middle;" height="20" width="20" /> [WebSocket Server CLI Example](./go/cmd/panrpc-example-websocket-server-cli/main.go)
    - [<img alt="Go" src="https://cdn.simpleicons.org/go" style="vertical-align: middle;" height="20" width="20" /> WebSocket Client CLI Example](./go/cmd/panrpc-example-websocket-client-cli/main.go)
    - [<img alt="typescript" src="https://cdn.simpleicons.org/typescript" style="vertical-align: middle;" height="20" width="20" /> WebSocket Server CLI Example](./ts/bin/panrpc-example-websocket-server-cli.ts)
    - [<img alt="typescript" src="https://cdn.simpleicons.org/typescript" style="vertical-align: middle;" height="20" width="20" /> WebSocket Client CLI Example](./ts/bin/panrpc-example-websocket-client-cli.ts)
    - [<img alt="typescript" src="https://cdn.simpleicons.org/typescript" style="vertical-align: middle;" height="20" width="20" /> WebSocket Client Web Example](./ts/bin/panrpc-example-websocket-client-web)
  - **WebRTC/weron (Stream-Oriented API)**
    - [<img alt="Go" src="https://cdn.simpleicons.org/go" style="vertical-align: middle;" height="20" width="20" /> WebRTC/weron Peer CLI Example](./go/cmd/panrpc-example-webrtc-peer-cli/main.go)
    - [<img alt="typescript" src="https://cdn.simpleicons.org/typescript" style="vertical-align: middle;" height="20" width="20" /> WebRTC/weron Peer CLI Example](./ts/bin/panrpc-example-webrtc-peer-cli.ts)
  - **Valkey/Redis (Message-Oriented API)**
    - [<img alt="Go" src="https://cdn.simpleicons.org/go" style="vertical-align: middle;" height="20" width="20" /> Valkey/Redis Server CLI Example](./go/cmd/panrpc-example-valkey-server-cli/main.go)
    - [<img alt="Go" src="https://cdn.simpleicons.org/go" style="vertical-align: middle;" height="20" width="20" /> Valkey/Redis Client CLI Example](./go/cmd/panrpc-example-valkey-client-cli/main.go)
    - [<img alt="typescript" src="https://cdn.simpleicons.org/typescript" style="vertical-align: middle;" height="20" width="20" /> Valkey/Redis Server CLI Example](./ts/bin/panrpc-example-valkey-server-cli.ts)
    - [<img alt="typescript" src="https://cdn.simpleicons.org/typescript" style="vertical-align: middle;" height="20" width="20" /> Valkey/Redis Client CLI Example](./ts/bin/panrpc-example-valkey-client-cli.ts)
- **Callbacks**
  - [<img alt="Go" src="https://cdn.simpleicons.org/go" style="vertical-align: middle;" height="20" width="20" /> Callbacks Demo Server CLI Example](./go/cmd/panrpc-example-tcp-callbacks-callee-cli/main.go)
  - [<img alt="Go" src="https://cdn.simpleicons.org/go" style="vertical-align: middle;" height="20" width="20" /> Callbacks Demo Client CLI Example](./go/cmd/panrpc-example-tcp-callbacks-caller-cli/main.go)
  - [<img alt="typescript" src="https://cdn.simpleicons.org/typescript" style="vertical-align: middle;" height="20" width="20" /> Callbacks Demo Server CLI Example](./ts/bin/panrpc-example-tcp-callbacks-callee-cli.ts)
  - [<img alt="typescript" src="https://cdn.simpleicons.org/typescript" style="vertical-align: middle;" height="20" width="20" /> Callbacks Demo Client CLI Example](./ts/bin/panrpc-example-tcp-callbacks-caller-cli.ts)
- **Closures**
  - [<img alt="Go" src="https://cdn.simpleicons.org/go" style="vertical-align: middle;" height="20" width="20" /> Closures Demo Server CLI Example](./go/cmd/panrpc-example-tcp-closures-callee-cli/main.go)
  - [<img alt="Go" src="https://cdn.simpleicons.org/go" style="vertical-align: middle;" height="20" width="20" /> Closures Demo Client CLI Example](./go/cmd/panrpc-example-tcp-closures-caller-cli/main.go)
  - [<img alt="typescript" src="https://cdn.simpleicons.org/typescript" style="vertical-align: middle;" height="20" width="20" /> Closures Demo Server CLI Example](./ts/bin/panrpc-example-tcp-closures-callee-cli.ts)
  - [<img alt="typescript" src="https://cdn.simpleicons.org/typescript" style="vertical-align: middle;" height="20" width="20" /> Closures Demo Client CLI Example](./ts/bin/panrpc-example-tcp-closures-caller-cli.ts)
- **Nested RPCs**
  - [<img alt="Go" src="https://cdn.simpleicons.org/go" style="vertical-align: middle;" height="20" width="20" /> Nested Server CLI Example](./go/cmd/panrpc-example-tcp-nested-server-cli/main.go)
  - [<img alt="Go" src="https://cdn.simpleicons.org/go" style="vertical-align: middle;" height="20" width="20" /> Nested Client CLI Example](./go/cmd/panrpc-example-tcp-nested-client-cli/main.go)
  - [<img alt="typescript" src="https://cdn.simpleicons.org/typescript" style="vertical-align: middle;" height="20" width="20" /> Nested Server CLI Example](./ts/bin/panrpc-example-tcp-nested-server-cli.ts)
  - [<img alt="typescript" src="https://cdn.simpleicons.org/typescript" style="vertical-align: middle;" height="20" width="20" /> Nested Client CLI Example](./ts/bin/panrpc-example-tcp-nested-client-cli.ts)
- **Benchmarks**
  - [<img alt="Go" src="https://cdn.simpleicons.org/go" style="vertical-align: middle;" height="20" width="20" /> Requests/Second Benchmark Server CLI Example](./go/cmd/panrpc-example-tcp-rps-server-cli/main.go)
  - [<img alt="Go" src="https://cdn.simpleicons.org/go" style="vertical-align: middle;" height="20" width="20" /> Requests/Second Benchmark Client CLI Example](./go/cmd/panrpc-example-tcp-rps-client-cli/main.go)
  - [<img alt="typescript" src="https://cdn.simpleicons.org/typescript" style="vertical-align: middle;" height="20" width="20" /> Requests/Second Benchmark Server CLI Example](./ts/bin/panrpc-example-tcp-rps-server-cli.ts)
  - [<img alt="typescript" src="https://cdn.simpleicons.org/typescript" style="vertical-align: middle;" height="20" width="20" /> Requests/Second Benchmark Client CLI Example](./ts/bin/panrpc-example-tcp-rps-client-cli.ts)
  - [<img alt="Go" src="https://cdn.simpleicons.org/go" style="vertical-align: middle;" height="20" width="20" /> Throughput Benchmark Server CLI Example](./go/cmd/panrpc-example-tcp-throughput-server-cli/main.go)
  - [<img alt="Go" src="https://cdn.simpleicons.org/go" style="vertical-align: middle;" height="20" width="20" /> Throughput Benchmark Client CLI Example](./go/cmd/panrpc-example-tcp-throughput-client-cli/main.go)
  - [<img alt="typescript" src="https://cdn.simpleicons.org/typescript" style="vertical-align: middle;" height="20" width="20" /> Throughput Benchmark Server CLI Example](./ts/bin/panrpc-example-tcp-throughput-server-cli.ts)
  - [<img alt="typescript" src="https://cdn.simpleicons.org/typescript" style="vertical-align: middle;" height="20" width="20" /> Throughput Benchmark Client CLI Example](./ts/bin/panrpc-example-tcp-throughput-client-cli.ts)

### Benchmarks

All benchmarks were conducted on a test machine with the following specifications:

| Property     | Value                                   |
| ------------ | --------------------------------------- |
| Device Model | Dell XPS 9320                           |
| OS           | Fedora release 38 (Thirty Eight) x86_64 |
| Kernel       | 6.3.11-200.fc38.x86_64                  |
| CPU          | 12th Gen Intel i7-1280P (20) @ 4.700GHz |
| Memory       | 31687MiB LPDDR5, 6400 MT/s              |

To reproduce the tests, see the [benchmark source code](#examples) and the [visualization source code](./docs/).

#### Requests/Second

> This is measured by calling RPCs with the different data types as the arguments.

<img src="./docs/rps.png" alt="Bar chart of the requests/second benchmark results for JSON and CBOR" width="550px">

| Data Type  | JSON (go) | CBOR (go) | JSON (typescript) | CBOR (typescript) |
| :--------- | --------: | --------: | ----------------: | ----------------: |
| array      |     75500 |     99683 |             57373 |             62848 |
| bool       |     79662 |    106226 |             57499 |             63324 |
| byte       |     81438 |    105916 |             57480 |             60169 |
| complex128 |       nan |       nan |             58849 |             59693 |
| complex64  |       nan |       nan |             58375 |             63018 |
| float32    |     79878 |    106359 |             54034 |             62068 |
| float64    |     78724 |    101498 |             55181 |             61987 |
| int        |     93569 |    119268 |             52115 |             59269 |
| int16      |     76995 |    104569 |             56596 |             62165 |
| int32      |     80425 |    106986 |             53847 |             63676 |
| int64      |     81276 |    101144 |             58126 |             64622 |
| int8       |     85734 |    113260 |             54081 |             60756 |
| rune       |     84113 |    109719 |             53753 |             61153 |
| slice      |     77975 |    101126 |             56404 |             62278 |
| string     |     77252 |    106265 |             57876 |             60453 |
| struct     |     77699 |    104968 |             57876 |             61498 |
| uint       |     81361 |    103698 |             58455 |             61729 |
| uint16     |     80990 |    106615 |             57004 |             62429 |
| uint32     |     80319 |    103672 |             55668 |             63651 |
| uint64     |     82412 |    107139 |             53627 |             63818 |
| uint8      |     82127 |    106076 |             59698 |             59955 |
| uintptr    |       nan |       nan |             53214 |             64170 |

#### Throughput

> This is measured by calling an RPC with `[]byte` as the argument.

<img src="./docs/throughput.png" alt="Bar chart of the throughput benchmark results for JSON and CBOR" width="550px">

| Serializer        | Average Throughput |
| ----------------- | ------------------ |
| CBOR (go)         | 1389 MB/s          |
| JSON (go)         | 105 MB/s           |
| CBOR (typescript) | 24 MB/s            |
| JSON (typescript) | 12 MB/s            |

### Protocol

The protocol used by panrpc is simple and independent of transport and serialization layer; in the following examples, we'll use JSON.

A function call to e.g. the `Println` function from above looks like this:

```json
{
  "request": {
    "call": "b3332cf0-4e50-4684-a909-05772e14595e",
    "function": "Println",
    "args": ["Hello, world!"]
  },
  "response": null
}
```

The request/response wrapper specifies whether the message is a function call (`request`) or return (`response`). `call` is the ID of the function call, as generated by the client; `function` is the function name and `args` is an array of the function's arguments.

A function return looks like this:

```json
{
  "request": null,
  "response": {
    "call": "b3332cf0-4e50-4684-a909-05772e14595e",
    "value": null,
    "err": ""
  }
}
```

Here, `response` specifies that the message is a function return. `call` is the ID of the function call from above, `value` is the function's return value, and the last element is the error message; `nil` errors are represented by the empty string.

Keep in mind that panrpc is bidirectional, meaning that both the client and server can send and receive both types of messages to each other.

### `purl` Command Line Arguments

```shell
$ purl --help
Like cURL, but for panrpc: Command-line tool for interacting with panrpc servers

Usage of purl:
	purl [flags] <(tcp|tls|unix|unixs|ws|wss|weron)://(host:port/function|path/function|password:key@community/channel[/remote]/function)> <[args...]>

Examples:
	purl tcp://localhost:1337/Increment '[1]'
	purl tls://localhost:443/Increment '[1]'
	purl unix:///tmp/panrpc.sock/Increment '[1]'
	purl unixs:///tmp/panrpc.sock/Increment '[1]'
	purl ws://localhost:1337/Increment '[1]'
	purl wss://localhost:443/Increment '[1]'
	purl weron://examplepass:examplekey@examplecommunity/panrpc.example.webrtc/Increment '[1]'

Flags:
  -listen
    	Whether to connect to remotes by listening or dialing (ignored for weron://)
  -serializer string
    	Serializer to use (json or cbor) (default "json")
  -timeout duration
    	Time to wait for a response to a call (default 10s)
  -tls-cert string
    	TLS certificate (only valid for tls://, unixs:// and wss://)
  -tls-key string
    	TLS key (only valid for tls://, unixs:// and wss://)
  -tls-verify
    	Whether to verify TLS peer certificates (only valid for tls://, unixs:// and wss://) (default true)
  -verbose
    	Whether to enable verbose logging
  -weron-force-relay
    	Force usage of TURN servers (only valid for weron://)
  -weron-ice string
    	Comma-separated list of STUN servers (in format stun:host:port) and TURN servers to use (in format username:credential@turn:host:port) (i.e. username:credential@turn:global.turn.twilio.com:3478?transport=tcp) (only valid for weron://) (default "stun:stun.l.google.com:19302")
  -weron-signaler string
    	Signaler address (only valid for weron://) (default "wss://weron.up.railway.app/")
```

## Acknowledgements

- [zserge/lorca](https://github.com/zserge/lorca) inspired the API design.

## Contributing

To contribute, please use the [GitHub flow](https://guides.github.com/introduction/flow/) and follow our [Code of Conduct](./CODE_OF_CONDUCT.md).

To build and start a development version of panrpc locally, run the following:

```shell
$ git clone https://github.com/pojntfx/panrpc.git

# For Go
$ cd panrpc/go
$ go run ./cmd/panrpc-example-tcp-server-cli/ # Starts the Go TCP example server CLI
# In another terminal
$ go run ./cmd/panrpc-example-tcp-client-cli/ # Starts the Go TCP example client CLI

# For TypeScript
$ cd panrpc/ts
$ npm install
$ npx tsx ./bin/panrpc-example-tcp-server-cli.ts # Starts the TypeScript TCP example server CLI
# In another terminal
$ npx tsx ./bin/panrpc-example-tcp-client-cli.ts # Starts the TypeScript TCP example client CLI
```

Have any questions or need help? Chat with us [on Matrix](https://matrix.to/#/#panrpc:matrix.org?via=matrix.org)!

## License

panrpc (c) 2024 Felicitas Pojtinger and contributors

SPDX-License-Identifier: Apache-2.0
