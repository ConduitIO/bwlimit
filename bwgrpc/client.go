// Copyright © 2023 Meroxa, Inc.
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package bwgrpc

import (
	"context"
	"net"
	"net/url"
	"strings"

	"github.com/conduitio/bwlimit"

	"google.golang.org/grpc"
)

// WithBandwidthLimitedContextDialer returns a DialOption that sets a dialer to
// create connections with a bandwidth limit. If FailOnNonTempDialError() is set
// to true, and an error is returned by f, gRPC checks the error's Temporary()
// method to decide if it should try to reconnect to the network address.
// A zero value for writeBytesPerSecond or readBytesPerSecond means the
// corresponding action will not have a bandwidth limit.
// This option can NOT be used together with grpc.WithContextDialer.
func WithBandwidthLimitedContextDialer(
	writeBytesPerSecond,
	readBytesPerSecond bwlimit.Byte,
	f func(context.Context, string) (net.Conn, error),
) grpc.DialOption {
	return grpc.WithContextDialer(func(ctx context.Context, addr string) (net.Conn, error) {
		conn, err := dial(ctx, addr, f)
		if err != nil {
			return nil, err
		}

		return bwlimit.NewConn(conn, writeBytesPerSecond, readBytesPerSecond), nil
	})
}

// dial uses the dialer to get a connection if supplied, and falls back to the
// default implementation used by google.golang.org/grpc.
func dial(ctx context.Context, addr string, dialer func(context.Context, string) (net.Conn, error)) (net.Conn, error) {
	if dialer != nil {
		return dialer(ctx, addr)
	}

	// default dialer copied from google.golang.org/grpc
	networkType, address := parseDialTarget(addr)
	return (&net.Dialer{}).DialContext(ctx, networkType, address)
}

// parseDialTarget returns the network and address to pass to dialer.
// Copied from google.golang.org/grpc.
func parseDialTarget(target string) (string, string) {
	net := "tcp"
	m1 := strings.Index(target, ":")
	m2 := strings.Index(target, ":/")
	// handle unix:addr which will fail with url.Parse
	if m1 >= 0 && m2 < 0 {
		if n := target[0:m1]; n == "unix" {
			return n, target[m1+1:]
		}
	}
	if m2 >= 0 {
		t, err := url.Parse(target)
		if err != nil {
			return net, target
		}
		scheme := t.Scheme
		addr := t.Path
		if scheme == "unix" {
			if addr == "" {
				addr = t.Host
			}
			return scheme, addr
		}
	}
	return net, target
}
