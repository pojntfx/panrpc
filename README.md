# dudirekta

![Logo](./docs/logo-readme.png)

Transport-agnostic framework that allows exposing and calling functions on both clients and servers.

[![hydrun CI](https://github.com/pojntfx/dudirekta/actions/workflows/hydrun.yaml/badge.svg)](https://github.com/pojntfx/dudirekta/actions/workflows/hydrun.yaml)
![Go Version](https://img.shields.io/badge/go%20version-%3E=1.18-61CFDD.svg)
[![Go Reference](https://pkg.go.dev/badge/github.com/pojntfx/dudirekta.svg)](https://pkg.go.dev/github.com/pojntfx/dudirekta)
[![npm CI](https://github.com/pojntfx/dudirekta/actions/workflows/npm.yaml/badge.svg)](https://github.com/pojntfx/dudirekta/actions/workflows/npm.yaml)
[![npm: @pojntfx/dudirekta](https://img.shields.io/npm/v/@pojntfx/dudirekta)](https://www.npmjs.com/package/@pojntfx/dudirekta)
[![TypeScript docs](https://img.shields.io/badge/TypeScript%20-docs-blue.svg)](https://pojntfx.github.io/dudirekta)
[![Matrix](https://img.shields.io/matrix/dudirekta:matrix.org)](https://matrix.to/#/#dudirekta:matrix.org?via=matrix.org)

## Overview

dudirekta is a novel RPC framework with a unique feature: It allows exposing functions on **both the client and server**!

It enables you to ...

- **Call remote functions transparently**: dudirekta makes use of reflection, so you can call functions as though they were local without defining your own protocol or generating code
- **Call functions on the client from the server**: Unlike most RPC frameworks, dudirekta allows for functions to be exposed on both the server and the client, enabling its use in new usecases such as doing bidirectional data transfer without subscriptions or pushing information before the client requests it
- **Implement RPCs on any transport layer**: By being able to work with any `io.ReadWriteCloser`, you can build services using dudirekta on pretty much any transport layer such as TCP, WebSockets or even WebRTC, meaning it can run in the browser!
- **Implement RPCs using any encoding/decoding layer**: Instead of depending on Protobuf or another fixed format for serialization, dudirekta supports almost any serialization framework, such as JSON or CBOR.
- **Pass closures and callbacks to RPCs**: Thanks to its bidirectional capabilities, dudirekta can use closures and callbacks transparently, just like a local function call!

## Installation

### Library

You can add dudirekta to your Go project by running the following:

```shell
$ go get github.com/pojntfx/dudirekta/...@latest
```

There is also a TypeScript version for browser and Node.js support (without transparent support for closures); you can install it like so:

```shell
$ npm i -s @pojntfx/dudirekta
```

This README's documentation only covers the Go version. For the TypeScript version, please check out [Hydrapp](https://github.com/pojntfx/hydrapp), it uses dudirekta in its examples; you can also find the complete package reference here: [![TypeScript docs](https://img.shields.io/badge/TypeScript%20-docs-blue.svg)](https://pojntfx.github.io/dudirekta) as well as example in [ts/dudirekta-example-websocket-client.ts](./ts/dudirekta-example-websocket-client.ts).

### `durl` Tool

In addition to the library, the CLI tool `durl` is also available; `durl` is like [cURL](https://curl.se/) or [gRPCurl](https://github.com/fullstorydev/grpcurl), but for dudirekta: A command-line tool for interacting with dudirekta servers.

Static binaries are available on [GitHub releases](https://github.com/pojntfx/dudirekta/releases).

On Linux, you can install them like so:

```shell
$ curl -L -o /tmp/durl "https://github.com/pojntfx/dudirekta/releases/latest/download/durl.linux-$(uname -m)"
$ sudo install /tmp/durl /usr/local/bin
```

On macOS, you can use the following:

```shell
$ curl -L -o /tmp/durl "https://github.com/pojntfx/dudirekta/releases/latest/download/durl.darwin-$(uname -m)"
$ sudo install /tmp/durl /usr/local/bin
```

On Windows, the following should work (using PowerShell as administrator):

```shell
PS> Invoke-WebRequest https://github.com/pojntfx/dudirekta/releases/latest/download/durl.windows-x86_64.exe -OutFile \Windows\System32\durl.exe
```

You can find binaries for more operating systems and architectures on [GitHub releases](https://github.com/pojntfx/dudirekta/releases).

## Usage

> TL;DR: Define the local and remote functions as struct methods, add them to a registry and link it with a transport

### 1. Define Local Functions

dudirekta uses reflection to create the glue code required to expose and call functions. Start by defining your server's exposed functions like so:

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

### 2. Define Remote Functions

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

### 3. Add Functions to a Registry

For the server, you can now create the registry, which will expose its functions:

```go
// server.go

registry := rpc.NewRegistry[remote, json.RawMessage](
	&local{},

	time.Second*10,
	context.Background(),
	nil,
)
```

And do the same for the client:

```go
// client.go

registry := rpc.NewRegistry[remote, json.RawMessage](
	&local{},

	time.Second*10,
	context.Background(),
	nil,
)
```

Note the second generic parameter; it is the type that should be used for encoding nested messages. For JSON, this is typically `json.RawMessage`, for CBOR, this is `cbor.RawMessage`. Using such a nested message type is recommended, as it leads to a faster encoding/decoding since it doesn't require multiple encoding/decoding steps in order to function, but using `[]byte` (which will use multiple encoding/decoding steps) is also possible if this is not an option (for more infromation, see [Protocol](#protocol)).

### 4. Link the Registry to a Transport and Serializer

Next, expose the functions by creating a TCP listener in your `main` func (you could also use WebSockets, WebRTC or anything that provides a `io.ReadWriteCloser`) and passing in your serialization framework:

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

### 5. Call the Functions

Now you can call the functions exposed on the server from the client and vise versa. For example, to call `Println`, a function exposed by the client from the server:

```go
// server.go

for _, peer := range registry.Peers() {
	if err := peer.Println(ctx, "Hello, world!"); err != nil {
		panic(err)
	}
}
```

Or to call the `Increment` function exposed by the server on the client:

```go
// client.go

for _, peer := range registry.Peers() {
	new, err := peer.Increment(ctx, 1)
	if err != nil {
		panic(err)
	}

	log.Println(new)
}
```

By passing the `Peers()` method to the local service itself, you can also access remote functions in the other direction:

```go
// server.go

type local struct {
	counter int64

	Peers func() map[string]remote
}

func (s *local) Increment(ctx context.Context, delta int64) (int64, error) {
	peerID := rpc.GetRemoteID(ctx)

	for candidateIP, peer := range s.Peers() {
		if candidateIP == peerID {
			peer.Println(ctx, fmt.Sprintf("Incrementing counter by %v", delta))
		}
	}

	return atomic.AddInt64(&s.counter, delta), nil
}

// In `main`:
service := &local{}
registry := rpc.NewRegistry(
	service,
	remote{},

	time.Second*10,
	context.Background(),
	nil,
)
service.Peers = registry.Peers
```

### 6. Using Closures and Callbacks

Because dudirekta is bidirectional, it is possible to pass closures and callbacks as function arguments, just like you would locally. For example, on the server:

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

for _, peer := range registry.Peers() {
	length, err := Iterate(peer, ctx, 5, func(i int, b string) (string, error) {
		log.Println("In iteration", i, b)

		return "This is from the caller", nil
	})
	if err != nil {
		panic(err)
	}

	log.Println(length)
}
```

🚀 That's it! We can't wait to see what you're going to build with dudirekta.

## Reference

### Examples

To make getting started with dudirekta easier, take a look at the following examples:

- **TCP Transport**
  - [TCP Server](./cmd/dudirekta-example-tcp-server/main.go)
  - [TCP Client](./cmd/dudirekta-example-tcp-client/main.go)
- **WebSocket Transport**
  - [WebSocket Server](./cmd/dudirekta-example-websocket-server/main.go)
  - [WebSocket Client](./cmd/dudirekta-example-websocket-client/main.go)
- **WebRTC Transport**
  - [WebRTC Peer](./cmd/dudirekta-example-webrtc-peer/main.go)
- **Callbacks**
  - [Callbacks Demo Server](./cmd/dudirekta-example-callbacks-callee/main.go)
  - [Callbacks Demo Client](./cmd/dudirekta-example-callbacks-caller/main.go)
- **Closures**
  - [Closures Demo Server](./cmd/dudirekta-example-closures-callee/main.go)
  - [Closures Demo Client](./cmd/dudirekta-example-closures-caller/main.go)

### Protocol

The protocol used by dudirekta is simple and based on JSONL.

A function call to e.g. the `Println` function from above looks like this:

```json
[true, "1", "Println", ["Hello, world!"]]
```

The first element specifies whether the message is a function call (`true`) or return (`false`). The second element is the ID of the function call, generated by the client; the third element is the function name and the last element is an array of the function's arguments serialized to JSON.

A function return looks like this:

```json
[false, "1", "", ""]
```

Here, the first element specifies that the message is a function return (`false`). The second element is the ID of the function call from above, the third element is the function's return value serialized to JSON, and the last element is the error message; `nil` errors are represented by the empty string.

Keep in mind that dudirekta is bidirectional, meaning that both the client and server can send and receive both types of messages to each other.

## Acknowledgements

- [zserge/lorca](https://github.com/zserge/lorca) inspired the API design.

## Contributing

To contribute, please use the [GitHub flow](https://guides.github.com/introduction/flow/) and follow our [Code of Conduct](./CODE_OF_CONDUCT.md).

To build and start a development version of dudirekta locally, run the following:

```shell
$ git clone https://github.com/pojntfx/dudirekta.git
$ cd dudirekta
$ go run ./cmd/dudirekta-example-tcp-server/ # Starts the TCP example server
# In another terminal
$ go run ./cmd/dudirekta-example-tcp-client/ # Starts the TCP example client
```

Have any questions or need help? Chat with us [on Matrix](https://matrix.to/#/#dudirekta:matrix.org?via=matrix.org)!

## License

dudirekta (c) 2023 Felicitas Pojtinger and contributors

SPDX-License-Identifier: Apache-2.0
