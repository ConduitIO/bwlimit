# BWLimit

[![License](https://img.shields.io/badge/license-Apache%202-blue)](https://github.com/ConduitIO/bwlimit/blob/main/LICENSE.md)
[![Test](https://github.com/ConduitIO/bwlimit/actions/workflows/test.yml/badge.svg)](https://github.com/ConduitIO/bwlimit/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/conduitio/bwlimit)](https://goreportcard.com/report/github.com/conduitio/bwlimit)
[![Go Reference](https://pkg.go.dev/badge/github.com/conduitio/bwlimit.svg)](https://pkg.go.dev/github.com/conduitio/bwlimit)

BWLimit lets you configure a bandwidth limit on [`net.Conn`](https://pkg.go.dev/net#Conn),
[`io.Reader`](https://pkg.go.dev/io#Reader) and [`io.Writer`](https://pkg.go.dev/io#Writer).

## Quick Start

BWLimit can be used to throttle the bandwidth (bytes per second) either on the
client or server.

Install it with:

```sh
go get github.com/conduitio/bwlimit
```

See usage examples below:
- [Server Side](#server-side)
- [Client Side](#client-side)
- [gRPC Client Interceptor](#grpc-client-interceptor)

### Server Side

To limit the bandwidth on the server use
[`bwlimit.NewListener`](https://pkg.go.dev/github.com/conduitio/bwlimit#NewListener).

```go
package main

import (
	"io"
	"log"
	"net"
	"net/http"

	"github.com/conduitio/bwlimit"
)

const (
	writeLimit = 1 * bwlimit.Mebibyte // write limit is 1048576 B/s
	readLimit  = 4 * bwlimit.KB       // read limit is 4000 B/s
)

func main() {
	ln, err := net.Listen("tcp", ":8080")
	if err != nil {
		log.Fatalf("Failed to listen: %v", err)
	}
	// limit the listener bandwidth
	ln = bwlimit.NewListener(ln, writeLimit, readLimit)

	http.Handle("/echo", http.HandlerFunc(echoHandler))
	srv := &http.Server{Addr: addr}
	log.Fatalf("Failed to serve: %v", srv.Serve(ln))
}

func echoHandler(w http.ResponseWriter, r *http.Request) {
	body, _ := io.ReadAll(r.Body)
	_, _ = w.Write(body)
}
```

### Client Side

To limit the bandwidth on the client use
[`bwlimit.NewDialer`](https://pkg.go.dev/github.com/conduitio/bwlimit#NewDialer).

```go
package main

import (
	"io"
	"net"
	"net/http"
	"time"

	"github.com/conduitio/bwlimit"
)

const (
	writeLimit = 1 * bwlimit.Mebibyte // write limit is 1048576 B/s
	readLimit  = 4 * bwlimit.KB       // read limit is 4000 B/s
)

func main() {
	// change dialer in the default transport to use a bandwidth limit
	dialer := bwlimit.NewDialer(&net.Dialer{
		Timeout:   30 * time.Second,
		KeepAlive: 30 * time.Second,
	}, writeLimit, readLimit)
	http.DefaultTransport.(*http.Transport).DialContext = dialer.DialContext

	// requests through the default client respect the bandwidth limit now
	resp, _ := http.DefaultClient.Get("http://localhost:8080/echo")
	_, _ = io.ReadAll(resp.Body)
}
```

### gRPC Client Interceptor

The gRPC interceptor is provided in a separate module, import it with:

```sh
go get github.com/conduitio/bwlimit/bwgrpc
```

To limit the bandwidth on a gRPC client use
[`bwgrpc.WithBandwidthLimitedContextDialer`](https://pkg.go.dev/github.com/conduitio/bwlimit/bwgrpc#WithBandwidthLimitedContextDialer).

```go
package main

import (
	"context"
	"log"

	"github.com/conduitio/bwlimit"
	"github.com/conduitio/bwlimit/bwgrpc"
	"github.com/conduitio/bwlimit/bwgrpc/testproto"
	"google.golang.org/grpc"
)

const (
	writeLimit = 1 * bwlimit.Mebibyte // write limit is 1048576 B/s
	readLimit  = 4 * bwlimit.KB       // read limit is 4000 B/s
)

func main() {
	// open connection with limited bandwidth
	conn, err := grpc.DialContext(
		context.Background(),
		"localhost:8080",
		// limit the bandwidth
		bwgrpc.WithBandwidthLimitedContextDialer(writeLimit, readLimit, nil),
	)
	if err != nil {
		log.Fatalf("Failed to dial: %v", err)
	}
	defer conn.Close()

	// create gRPC client with the limited connection
	c := testproto.NewTestServiceClient(conn)
	
	// use client to send request
	_, err = c.TestRPC(ctx, &testproto.TestRequest{})
	if err != nil {
		log.Fatalf("Failed to send RPC: %v", err)
	}
}
```