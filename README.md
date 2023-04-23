# BWLimit

[![License](https://img.shields.io/badge/license-Apache%202-blue)](https://github.com/ConduitIO/bwlimit/blob/main/LICENSE.md)
[![Test](https://github.com/ConduitIO/bwlimit/actions/workflows/test.yml/badge.svg)](https://github.com/ConduitIO/bwlimit/actions/workflows/test.yml)
[![Go Report Card](https://goreportcard.com/badge/github.com/conduitio/bwlimit)](https://goreportcard.com/report/github.com/conduitio/bwlimit)

BWLimit lets you configure a bandwidth limit on [`net.Conn`](https://pkg.go.dev/net#Conn).

## Quick Start

BWLimit can be used to throttle the bandwidth either on the client or server. It
can throttle any `net.Conn` connection.

### Server side

To limit the bandwidth on the server use `bwlimit.NewListener`.

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

### Client side

### gRPC Client Interceptor

The gRPC interceptor is provided in a separate module, import it with:

```
go get github.com/conduitio/bwlimit/bwgrpc
```

Example usage:

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