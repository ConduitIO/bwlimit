package ratelimit

import (
	"context"
	"net"
	"net/url"
	"strings"

	"google.golang.org/grpc"
)

func WithRateLimitedContextDialer(writeBytesPerSecond int, readBytesPerSecond int, dialer func(ctx context.Context, s string) (net.Conn, error)) grpc.DialOption {
	return grpc.WithContextDialer(func(ctx context.Context, s string) (net.Conn, error) {
		var conn net.Conn
		var err error
		if dialer != nil {
			conn, err = dialer(ctx, s)
		} else {
			// default dialer copied from google.golang.org/grpc
			networkType, address := parseDialTarget(s)
			conn, err = (&net.Dialer{}).DialContext(ctx, networkType, address)
		}
		if err != nil {
			return nil, err
		}

		return NewRateLimitedConn(conn, writeBytesPerSecond, readBytesPerSecond), nil
	})
}

// parseDialTarget returns the network and address to pass to dialer. Copied from google.golang.org/grpc.
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
