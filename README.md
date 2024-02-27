# panrpc

<picture>
  <source media="(prefers-color-scheme: dark)" srcset="./docs/logo-readme-dark.png">
  <img alt="Logo" src="./docs/logo-readme-light.png">
</picture>

Language-, transport- and serialization-agnostic RPC framework with remote closure support that allows exposing and calling functions on both clients and servers.

[![hydrun CI](https://github.com/pojntfx/panrpc/actions/workflows/hydrun.yaml/badge.svg)](https://github.com/pojntfx/panrpc/actions/workflows/hydrun.yaml)
![Go Version](https://img.shields.io/badge/go%20version-%3E=1.18-61CFDD.svg)
[![Go Reference](https://pkg.go.dev/badge/github.com/pojntfx/panrpc.svg)](https://pkg.go.dev/github.com/pojntfx/panrpc)
[![npm CI](https://github.com/pojntfx/panrpc/actions/workflows/npm.yaml/badge.svg)](https://github.com/pojntfx/panrpc/actions/workflows/npm.yaml)
[![npm: @pojntfx/panrpc](https://img.shields.io/npm/v/@pojntfx/panrpc)](https://www.npmjs.com/package/@pojntfx/panrpc)
[![TypeScript docs](https://img.shields.io/badge/TypeScript%20-docs-blue.svg)](https://pojntfx.github.io/panrpc)
[![Matrix](https://img.shields.io/matrix/panrpc:matrix.org)](https://matrix.to/#/#panrpc:matrix.org?via=matrix.org)

## Overview

panrpc is a novel RPC framework with a unique feature: It allows exposing functions on **both the client and server**!

It enables you to ...

- **Call remote functions transparently**: panrpc makes use of reflection, so you can call functions as though they were local without defining your own protocol or generating code
- **Call functions on the client from the server**: Unlike most RPC frameworks, panrpc allows for functions to be exposed on both the server and the client, enabling its use in new usecases such as doing bidirectional data transfer without subscriptions or pushing information before the client requests it
- **Implement RPCs on any transport layer**: By being able to work with any `io.ReadWriteCloser` such as TCP, WebSocket or WebRTC with the [Stream-Oriented API](https://pkg.go.dev/github.com/pojntfx/panrpc/pkg/rpc#LinkStream), or any message-based transport such as Redis or NATS with the [Message-Oriented API](https://pkg.go.dev/github.com/pojntfx/panrpc/pkg/rpc#LinkMessage), you can use panrpc to build services that run in almost any environment, including the browser!
- **Use an encoding/decoding layer of your choice**: Instead of depending on Protobuf or another fixed format for serialization, panrpc can work with every serialization framework that implements the basic `Marshal`/`Unmarshal` interface, such as JSON or CBOR.
- **Pass closures and callbacks to RPCs**: Thanks to its bidirectional capabilities, panrpc can handle closures and callbacks transparently, just like with local function calls!

## Installation

### Library

You can add panrpc to your **Go** project by running the following:

```shell
go get github.com/pojntfx/panrpc/...@latest
```

For **TypeScript**, you can add panrpc to your project (both server-side TypeScript/Node.js and all major browser engines are supported) by running the following:

```shell
npm install @pojntfx/panrpc
```

### `purl` Tool

In addition to the library, the CLI tool `purl` is also available; `purl` is like [cURL](https://curl.se/) and [gRPCurl](https://github.com/fullstorydev/grpcurl), but for panrpc: A command-line tool for interacting with panrpc servers. `purl` is provided in the form of static binaries.

On Linux, you can install them like so:

```shell
curl -L -o /tmp/purl "https://github.com/pojntfx/panrpc/releases/latest/download/purl.linux-$(uname -m)"
sudo install /tmp/purl /usr/local/bin
```

On macOS, you can use the following:

```shell
curl -L -o /tmp/purl "https://github.com/pojntfx/panrpc/releases/latest/download/purl.darwin-$(uname -m)"
sudo install /tmp/purl /usr/local/bin
```

On Windows, the following should work (using PowerShell as administrator):

```PowerShell
Invoke-WebRequest https://github.com/pojntfx/panrpc/releases/latest/download/purl.windows-x86_64.exe -OutFile \Windows\System32\purl.exe
```

You can find binaries for more operating systems and architectures on [GitHub releases](https://github.com/pojntfx/panrpc/releases).

## Usage

### Go

<details>
  <summary>Expand instructions</summary>

> TL;DR: Define the local and remote functions as struct methods, add them to a registry and link it with a transport

#### 1. Define Local Functions

panrpc uses reflection to create the glue code required to expose and call functions. Start by defining your server's exposed functions like so:

```go
// server.go

type local struct {
	counter int64
}

func (s *local) Increment(ctx context.Context, delta int64) (int64, error) {
	log.Println("Incrementing counter by", delta, "for peer with ID", rpc.GetRemoteID(ctx))

	return atomic.AddInt64(&s.counter, delta), nil
}
```

In your client, define the exposed functions like so:

```go
// client.go

type local struct{}

func (s *local) Println(ctx context.Context, msg string) error {
	log.Println("Printing message", msg, "for peer with ID", rpc.GetRemoteID(ctx))

	fmt.Println(msg)

	return nil
}
```

The following limitations on which functions you can expose exist:

- Functions must have `context.Context` as their first argument
- Functions can not have variadic arguments
- Functions must return either an error or a single value and an error

#### 2. Define Remote Functions

Next, define the functions exposed by the client to the server using a struct without method implementations:

```go
// server.go

type remote struct {
	Println func(ctx context.Context, msg string) error
}
```

And do the same for the client:

```go
// client.go

type remote struct {
	Increment func(ctx context.Context, delta int64) (int64, error)
}
```

#### 3. Add Functions to a Registry

For the server, you can now create the registry, which will expose its functions:

```go
// server.go

registry := rpc.NewRegistry[remote, json.RawMessage](
	&local{},

	context.Background(),

	nil,
)
```

And do the same for the client:

```go
// client.go

registry := rpc.NewRegistry[remote, json.RawMessage](
	&local{},

	context.Background(),

	nil,
)
```

Note the second generic parameter; it is the type that should be used for encoding nested messages. For JSON, this is typically `json.RawMessage`, for CBOR, this is `cbor.RawMessage`. Using such a nested message type is recommended, as it leads to a faster encoding/decoding since it doesn't require multiple encoding/decoding steps in order to function, but using `[]byte` (which will use multiple encoding/decoding steps) is also possible if this is not an option (for more infromation, see [Protocol](#protocol)).

#### 4. Link the Registry to a Transport and Serializer

Next, expose the functions by linking them to a transport. There are two available transport APIs; the [Stream-Oriented API](https://pkg.go.dev/github.com/pojntfx/panrpc/pkg/rpc#LinkStream) (which is useful for stream-like transports such as TCP, WebSockets, WebRTC or anything else that provides an `io.ReadWriteCloser`), and the [Message-Oriented API](https://pkg.go.dev/github.com/pojntfx/panrpc/pkg/rpc#LinkMessage) (which is useful for transports that use messages, such as message brokers like Redis, UDP or other packet-based protocols). In this example, we'll use the stream-oriented API; for more information on using the m, meaning it can run in the browser!essage-oriented API, see [Examples](#examples).

Similarly so, as mentioned in [Add Functions to a Registry](#3-add-functions-to-a-registry), it is possible to use almost any serialization framework you want, as long as it can provide the necessary import interface. In this example, we'll be using the `encoding/json` package from the Go standard library, but in most cases, a more performant and compact framework such as CBOR is the better choice. See [Benchmarks](#benchmarks) for usage examples with other serialization frameworks and a performance comparison.

Start by creating a TCP listener in your `main` func (you could also use WebSockets, WebRTC or anything that provides a `io.ReadWriteCloser`) and passing in your serialization framework:

```go
// server.go

lis, err := net.Listen("tcp", "localhost:1337")
if err != nil {
	panic(err)
}
defer lis.Close()

for {
	func() {
		conn, err := lis.Accept()
		if err != nil {
			return
		}

		go func() {
			defer func() {
				_ = conn.Close()

				if err := recover(); err != nil {
					log.Printf("Client disconnected with error: %v", err)
				}
			}()

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
			}()
	}()
}
```

For the client, do the same, except this time connect to the server by dialing it:

```go
// client.go

conn, err := net.Dial("tcp", *addr)
if err != nil {
	panic(err)
}
defer conn.Close()

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
```

#### 5. Call the Functions

Now you can call the functions exposed on the server from the client and vise versa. For example, to call `Println`, a function exposed by the client from the server:

```go
// server.go

if err := registry.ForRemotes(func(remoteID string, remote remote) error {
	return remote.Println(ctx, "Hello, world!")
}); err != nil {
	panic(err)
}
```

Or to call the `Increment` function exposed by the server on the client:

```go
// client.go

if err := registry.ForRemotes(func(remoteID string, remote remote) error {
	new, err := remote.Increment(ctx, 1)
	if err != nil {
		return err
	}

	log.Println(new)
}); err != nil {
	panic(err)
}
```

By passing the `ForRemotes()` method to the local service itself, you can also access remote functions in the other direction:

```go
// server.go

type local struct {
	counter int64

	ForRemotes func(cb func(remoteID string, remote R) error) error
}

func (s *local) Increment(ctx context.Context, delta int64) (int64, error) {
	remoteID := rpc.GetRemoteID(ctx)

	if err := registry.ForRemotes(func(candidateID string, remote remote) error {
		if candidateID == remoteID {
			return peer.Println(ctx, fmt.Sprintf("Incrementing counter by %v", delta))
		}
	}); err != nil {
		return -1, err
	}

	return atomic.AddInt64(&s.counter, delta), nil
}

// In `main`:
service := &local{}
registry := rpc.NewRegistry[remote, json.RawMessage](
	service,

	context.Background(),

	nil,
)
service.ForRemotes = registry.ForRemotes
```

#### 6. Using Closures and Callbacks

Because panrpc is bidirectional, it is possible to pass closures and callbacks as function arguments, just like you would locally. For example, on the server:

```go
// server.go

type local struct{}

func (s *local) Iterate(
	ctx context.Context,
	length int,
	onIteration func(i int, b string) (string, error),
) (int, error) {
	for i := 0; i < length; i++ {
		rv, err := onIteration(i, "This is from the callee")
		if err != nil {
			return -1, err
		}

		log.Println("Closure returned:", rv)
	}

	return length, nil
}

type remote struct{}
```

And the client:

```go
// client.go

type local struct{}

type remote struct {
	Iterate func(
		ctx context.Context,
		length int,
		onIteration func(i int, b string) (string, error),
	) (int, error)
}
```

When you call `peer.Iterate`, you can now pass in a closure:

```go
// client.go

if err := registry.ForRemotes(func(remoteID string, remote remote) error {
	length, err := remote.Iterate(ctx, 5, func(i int, b string) (string, error) {
		log.Println("In iteration", i, b)

		return "This is from the caller", nil
	})
	if err != nil {
		return err
	}

	log.Println(length)
}); err != nil {
	panic(err)
}
```

🚀 That's it! We can't wait to see what you're going to build with panrpc. Be sure to take a look at the [reference](#reference) and [examples](#examples) for more information.

</details>

### TypeScript

<details>
  <summary>Expand instructions</summary>

> Just looking for sample code? Check out the sources for the example [coffee machine server](./ts/bin/panrpc-example-websocket-coffee-server-cli.ts) and [coffee machine client/remote control](./ts/bin/panrpc-example-websocket-coffee-client-cli.ts).

#### 1. Choosing a Transport and Serializer

Start by creating a new npm module for the tutorial and installing `@pojntfx/panrpc`:

```shell
mkdir -p panrpc-tutorial
cd panrpc-tutorial
npm init -y
npm install @pojntfx/panrpc
```

The TypeScript version of panrpc supports many transports. While common ones are TCP, WebSockets, UNIX sockets or WebRTC, anything that directly implements or can be adapted to a [WHATWG stream](https://developer.mozilla.org/en-US/docs/Web/API/Streams_API) can be used with the panrpc [`linkStream` API](https://pojntfx.github.io/panrpc/classes/Registry.html#linkStream). If you want to use a message broker like Redis or NATS as the transport, or need more control over the wire protocol, you can use the [`linkMessage` API](https://pojntfx.github.io/panrpc/classes/Registry.html#linkMessage) instead. For this tutorial, we'll be using WebSockets as the transport through the `ws` library, which you can install like so:

```shell
npm install ws
```

In addition to supporting many transports, the TypeScript version of panrpc also supports different serializers. Common ones are JSON and CBOR, but similarly to transports anything that implements or can be adapted to a WHATWG stream can be used. For this tutorial, we'll be using JSON as the serializer through the `@streamparser/json-whatwg` library, which you can install like so:

```shell
npm install @streamparser/json-whatwg
```

#### 2. Creating a Server

- For this example we're creating a coffee machine server
- Creating the service with the RPCs (simple unary `BrewCoffee` RPC that simulates brewing coffee by waiting for 5 seconds before returning the new water level)
- Note how errors can simply be thrown and how no manual serialization needs to happen
- Adding the service to the RPC registry
- Creating a WebSocket server
- Creating a JSON encoder/decoder stream
- Passing the encoder/decoder streams to `registry.linkStream`
- Starting the server and watching it listen

#### 3. Creating a Client

- We're creating a remote control CLI that can control the coffee machine server
- Client will simply call the `BrewCoffee` function
- Defining the remote coffee machine server's RPCs
- Adding the remote RPCs to the RPC registry
- Creating a WebSocket client
- Creating a JSON encode/decoder stream
- Passing the encoder/decoder streams to `registry.linkStream`
- Starting the client and watching it connect

#### 4. Calling the Server's RPCs from the Client

- Client and server are connected, but we aren't calling the RPCs yet
- We'll create a simple TUI that will read a letter from `stdin` and brew a specific variant and size of coffee
- We'll then print the remaining water level back to the terminal
- Starting the client again, pressing "a", watching the new level being returned to the terminal

#### 5. Calling the Client's RPCs from the Server

- So far we've added RPCs to the server and called them from the client, like with most RPC frameworks
- Now we'll add RPCs to the client that the server can call
- This RPC will be called on all remote controls that are connected to the coffee machine to let them know that the coffee machine is currently brewing coffee
- We'll start by creating a service with a `SetCoffeeMachineBrewing` RPC that the client will expose (simply logs to the console)
- Add this service to the client's registry
- On the server's `BrewCoffee` RPC, before brewing coffee and after finishing, we'll call `SetCoffeeMachine` RPC for all clients that are connected (except the one that is calling the RPC itself - we can do that by checking the client's ID) with `this.forRemotes?`
- We can set `forRemotes` by getting it from the registry
- Starting the server again and starting three clients, then pressing "a" on one of them, and watching the other connected clients logging the brewing state before it has started/after it has stopped brewing except on the client that requested coffee to be brewed

#### 6. Passing Closures to RPCs

- In addition to calling RPCs on the client from the server and vice-versa, we can also pass functions (closures) to RPCs as arguments like with local functions and call them on the other end like if they were local
- We'll use this to report back progress of the coffee brewing process to the client while the `BrewCoffee` RPC is being called instead of just returning after it is done
- Add `onProgress` argument to remote `BrewCoffee` service on coffee machine server and decorate it with `@remoteClosure` to let `panrpc` know that it should be a closure
- Call the `onProgress` closure during the call to report progress
- On the client, extend the remote coffee machine service with the new remote closure argument, and pass along an anonymous function that logs the progress to `stdout`
- Restart the server and client
- Press "a", instead of just returning the water level after it's done it will stream in the progress as a percentage one by one

🚀 That's it! We can't wait to see what you're going to build with panrpc. Be sure to take a look at the [reference](#reference) and [examples](#examples) for more information.

</details>

## Reference

### Library API

- [![Go Reference](https://pkg.go.dev/badge/github.com/pojntfx/panrpc.svg)](https://pkg.go.dev/github.com/pojntfx/panrpc)
- [![TypeScript docs](https://img.shields.io/badge/TypeScript%20-docs-blue.svg)](https://pojntfx.github.io/panrpc)

### Examples

To make getting started with panrpc easier, take a look at the following examples:

- **Transports**
  - **TCP (Stream-Oriented API)**
    - [Go TCP Server CLI Example](./go/cmd/panrpc-example-tcp-server-cli/main.go)
    - [Go TCP Client CLI Example](./go/cmd/panrpc-example-tcp-client-cli/main.go)
    - [TypeScript TCP Server CLI Example](./ts/bin/panrpc-example-tcp-server-cli.ts)
    - [TypeScript TCP Client CLI Example](./ts/bin/panrpc-example-tcp-client-cli.ts)
  - **UNIX Socket (Stream-Oriented API)**
    - [Go UNIX Socket Server CLI Example](./go/cmd/panrpc-example-unix-server-cli/main.go)
    - [Go UNIX Socket Client CLI Example](./go/cmd/panrpc-example-unix-client-cli/main.go)
    - [TypeScript UNIX Socket Server CLI Example](./ts/bin/panrpc-example-unix-server-cli.ts)
    - [TypeScript UNIX Socket Client CLI Example](./ts/bin/panrpc-example-unix-client-cli.ts)
  - **`stdin/stdout` Pipe (Stream-Oriented API)**
    - [Go `stdin/stdout` Pipe Socket Server CLI Example](./go/cmd/panrpc-example-pipe-server-cli/main.go)
    - [Go `stdin/stdout` Pipe Socket Client CLI Example](./go/cmd/panrpc-example-pipe-client-cli/main.go)
    - [TypeScript `stdin/stdout` Pipe Server CLI Example](./ts/bin/panrpc-example-pipe-server-cli.ts)
    - [TypeScript `stdin/stdout` Pipe Client CLI Example](./ts/bin/panrpc-example-pipe-client-cli.ts)
  - **WebSocket (Stream-Oriented API)**
    - [Go WebSocket Server CLI Example](./go/cmd/panrpc-example-websocket-server-cli/main.go)
    - [Go WebSocket Client CLI Example](./go/cmd/panrpc-example-websocket-client-cli/main.go)
    - [TypeScript WebSocket Server CLI Example](./ts/bin/panrpc-example-websocket-server-cli.ts)
    - [TypeScript WebSocket Client CLI Example](./ts/bin/panrpc-example-websocket-client-cli.ts)
    - [TypeScript WebSocket Client Web Example](./ts/bin/panrpc-example-websocket-client-web)
  - **WebRTC (Stream-Oriented API)**
    - [Go WebRTC Peer CLI Example](./go/cmd/panrpc-example-webrtc-peer-cli/main.go)
    - [TypeScript WebRTC Peer CLI Example](./ts/bin/panrpc-example-webrtc-peer-cli.ts)
  - **Redis (Message-Oriented API)**
    - [Go Redis Server CLI Example](./go/cmd/panrpc-example-redis-server-cli/main.go)
    - [Go Redis Client CLI Example](./go/cmd/panrpc-example-redis-client-cli/main.go)
    - [TypeScript Redis Server CLI Example](./ts/bin/panrpc-example-redis-server-cli.ts)
    - [TypeScript Redis Client CLI Example](./ts/bin/panrpc-example-redis-client-cli.ts)
- **Callbacks**
  - [Go Callbacks Demo Server CLI Example](./go/cmd/panrpc-example-callbacks-callee-cli/main.go)
  - [Go Callbacks Demo Client CLI Example](./go/cmd/panrpc-example-callbacks-caller-cli/main.go)
  - [TypeScript Callbacks Demo Server CLI Example](./ts/bin/panrpc-example-callbacks-callee-cli.ts)
  - [TypeScript Callbacks Demo Client CLI Example](./ts/bin/panrpc-example-callbacks-caller-cli.ts)
- **Closures**
  - [Go Closures Demo Server CLI Example](./go/cmd/panrpc-example-closures-callee-cli/main.go)
  - [Go Closures Demo Client CLI Example](./go/cmd/panrpc-example-closures-caller-cli/main.go)
  - [TypeScript Closures Demo Server CLI Example](./ts/bin/panrpc-example-closures-callee-cli.ts)
  - [TypeScript Closures Demo Client CLI Example](./ts/bin/panrpc-example-closures-caller-cli.ts)
- **Benchmarks**
  - [Go Requests/Second Benchmark Server CLI Example](./go/cmd/panrpc-example-tcp-rps-server-cli/main.go)
  - [Go Requests/Second Benchmark Client CLI Example](./go/cmd/panrpc-example-tcp-rps-client-cli/main.go)
  - [TypeScript Requests/Second Benchmark Server CLI Example](./ts/bin/panrpc-example-tcp-rps-server-cli.ts)
  - [TypeScript Requests/Second Benchmark Client CLI Example](./ts/bin/panrpc-example-tcp-rps-client-cli.ts)
  - [Go Throughput Benchmark Server CLI Example](./go/cmd/panrpc-example-tcp-throughput-server-cli/main.go)
  - [Go Throughput Benchmark Client CLI Example](./go/cmd/panrpc-example-tcp-throughput-client-cli/main.go)
  - [TypeScript Throughput Benchmark Server CLI Example](./ts/bin/panrpc-example-tcp-throughput-server-cli.ts)
  - [TypeScript Throughput Benchmark Client CLI Example](./ts/bin/panrpc-example-tcp-throughput-client-cli.ts)

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
git clone https://github.com/pojntfx/panrpc.git

# For Go
cd panrpc/go
go run ./cmd/panrpc-example-tcp-server-cli/ # Starts the Go TCP example server CLI
# In another terminal
go run ./cmd/panrpc-example-tcp-client-cli/ # Starts the Go TCP example client CLI

# For TypeScript
cd panrpc/ts
npm install
npx tsx ./bin/panrpc-example-tcp-server-cli.ts # Starts the TypeScript TCP example server CLI
# In another terminal
npx tsx ./bin/panrpc-example-tcp-client-cli.ts # Starts the TypeScript TCP example client CLI
```

Have any questions or need help? Chat with us [on Matrix](https://matrix.to/#/#panrpc:matrix.org?via=matrix.org)!

## License

panrpc (c) 2023 Felicitas Pojtinger and contributors

SPDX-License-Identifier: Apache-2.0
