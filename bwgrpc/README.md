# BWLimit gRPC

This module provides a `grpc.DialOption` for constructing a gRPC connection with
a bandwidth limit. See
[github.com/ConduitIO/bwlimit](https://github.com/ConduitIO/bwlimit) for more
information.

## Quick Start

Install it using:

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
