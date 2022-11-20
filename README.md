# dudirekta

![Logo](./docs/logo-readme.png)

Transport-agnostic framework that allows exposing and calling functions on both clients and servers.

![Go Version](https://img.shields.io/badge/go%20version-%3E=1.18-61CFDD.svg)
[![Go Reference](https://pkg.go.dev/badge/github.com/pojntfx/dudirekta.svg)](https://pkg.go.dev/github.com/pojntfx/dudirekta)
[![Matrix](https://img.shields.io/matrix/dudirekta:matrix.org)](https://matrix.to/#/#dudirekta:matrix.org?via=matrix.org)

## Overview

dudirekta is a novel RPC framework with a unique feature: It allows exposing functions on **both the client and server**!

It enables you to ...

- **Call remote functions transparently**: dudirekta makes use of reflection, so you can call functions as though they were local without defining your own protocol or generating code
- **Call functions on the client from the server**: Unlike most RPC frameworks, dudirekta allows for functions to be exposed on both the server and the client, enabling its use in new usecases such as doing bidirectional data transfer without subscriptions or pushing information before the client requests it
- **Implement RPCs on any transport layer**: By being able to work with any `io.ReadWriteCloser`, you can build services using dudirekta on pretty much any transport layer such as TCP, WebSockets or even WebRTC, meaning it can run in the browser!

## Installation

You can add dudirekta to your Go project by running the following:

```shell
$ go get github.com/pojntfx/dudirekta/...@latest
```

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

registry := rpc.NewRegistry(
	&local{},
	remote{},

	time.Second*10,
	context.Background(),
)
```

### 4. Link the Registry to a Transport

Next, expose the functions by creating a TCP listener in your `main` func (you could also use WebSockets, WebRTC or anything that provides a `io.ReadWriteCloser`):

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

			if err := registry.Link(conn); err != nil {
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

if err := registry.Link(conn); err != nil {
	panic(err)
}
```

### 5. Call the Functions

Now you can call the functions exposed on the server from the client and vise versa. For example, to call `Println`, a function exposed by the client from the server:

```go
// server.go

time.AfterFunc(time.Second*10, func() {
	for _, peer := range registry.Peers() {
		if err := peer.Println(ctx, "Hello, world!"); err != nil {
			panic(err)
		}
	}
})
```

Or to call the `Increment` function exposed by the server on the client:

```go
// client.go

time.AfterFunc(time.Second*10, func() {
	for _, peer := range registry.Peers() {
		new, err := peer.Increment(ctx, 1)
		if err != nil {
			panic(err)
		}

		log.Println(new)
	}
})
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
  - [Browser Client](./cmd/dudirekta-example-websocket-client/index.html)
- **WebRTC Transport**
  - [WebRTC Peer](./cmd/dudirekta-example-webrtc-peer/main.go)

Be sure to also check out [Hydrapp](https://github.com/pojntfx/hydrapp), a framework for building apps that run everywhere - it uses dudirekta extensively.

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
- All the rest of the authors who worked on the dependencies used! Thanks a lot!

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

dudirekta (c) 2022 Felicitas Pojtinger and contributors

SPDX-License-Identifier: AGPL-3.0
