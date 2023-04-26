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
	"log"
	"net"
	"sync"
	"testing"
	"time"

	"github.com/conduitio/bwlimit/bwgrpc/testproto"
	"github.com/golang/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestInterceptorBandwidthLimit(t *testing.T) {
	// use in-memory connection
	lis := bufconn.Listen(1024 * 1024)
	dialer := func(ctx context.Context, _ string) (net.Conn, error) {
		return lis.DialContext(ctx)
	}

	// prepare server
	wantResp := &testproto.TestResponse{Code: 1}
	startTestServer(t, lis, wantResp)

	// prepare client with write bandwidth limit of 100 B/s
	c := newTestClient(t, WithBandwidthLimitedContextDialer(100, 0, dialer))

	// send a request and measure how long it takes to get a response
	before := time.Now()
	resp, err := c.TestRPC(context.Background(), &testproto.TestRequest{Id: "abcdefghijklmnopqrstuvwxyz"})
	gotDelay := time.Since(before)
	if err != nil {
		t.Fatalf("Failed to call TestRPC: %v", err)
	}

	if resp.Code != wantResp.Code {
		t.Fatalf("responses don't match, expected %v, got %v", resp, wantResp)
	}

	// check how long it took
	const (
		// It should take 0.55 seconds, since we need to write 155 bytes and are rate limited to 100 B/s
		// After writing 100 bytes we delay for 550ms to write the remaining 55 bytes
		wantDelay = 550 * time.Millisecond
		// Allow 10 milliseconds of difference
		epsilon = 10 * time.Millisecond
	)

	gotDiff := gotDelay - wantDelay
	if gotDiff < 0 {
		gotDiff = -gotDiff
	}

	if gotDiff > epsilon {
		t.Fatalf("expected a maximum delay of %v, got %v, allowed epsilon of %v exceeded", wantDelay, gotDelay, epsilon)
	}
}

func startTestServer(t *testing.T, lis net.Listener, resp *testproto.TestResponse) {
	ctrl := gomock.NewController(t)
	srv := grpc.NewServer()

	// create and register simple mock server
	mockServer := testproto.NewMockTestServiceServer(ctrl)
	mockServer.EXPECT().
		TestRPC(gomock.Any(), gomock.Any()).
		DoAndReturn(
			func(ctx context.Context, request *testproto.TestRequest) (*testproto.TestResponse, error) {
				return resp, nil
			},
		)
	testproto.RegisterTestServiceServer(srv, mockServer)

	// start gRPC server
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		if err := srv.Serve(lis); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()
	t.Cleanup(func() {
		srv.GracefulStop()
		wg.Wait()
	})
}

func newTestClient(t *testing.T, dialerOption grpc.DialOption) testproto.TestServiceClient {
	conn, err := grpc.DialContext(
		context.Background(),
		"bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		// this interceptor limits the bandwidth
		dialerOption,
	)
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	t.Cleanup(func() {
		conn.Close()
	})

	return testproto.NewTestServiceClient(conn)
}
