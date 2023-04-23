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

package bwlimit

import (
	"context"
	"net"
	"net/http"
)

func TransportWithBandwidthLimit(transport *http.Transport, writeLimitPerSecond, readLimitPerSecond Byte) {
	setDialContext(transport, writeLimitPerSecond, readLimitPerSecond)
	setDialTLSContext(transport, writeLimitPerSecond, readLimitPerSecond)
}

func setDialContext(transport *http.Transport, writeLimitPerSecond, readLimitPerSecond Byte) {
	dial := transport.DialContext
	if dial == nil {
		dial = (&net.Dialer{}).DialContext
	}
	transport.DialContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := dial(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		return NewConn(conn, writeLimitPerSecond, readLimitPerSecond), nil
	}
}

func setDialTLSContext(transport *http.Transport, writeLimitPerSecond, readLimitPerSecond Byte) {
	if transport.DialTLSContext == nil {
		return // nothing to overwrite, the transport will use DialContext
	}
	dial := transport.DialTLSContext
	transport.DialTLSContext = func(ctx context.Context, network, addr string) (net.Conn, error) {
		conn, err := dial(ctx, network, addr)
		if err != nil {
			return nil, err
		}
		return NewConn(conn, writeLimitPerSecond, readLimitPerSecond), nil
	}

}
