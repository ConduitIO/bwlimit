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

	"github.com/conduitio/bwlimit"
	"github.com/conduitio/bwlimit/bwgrpc/testproto"
	"github.com/golang/mock/gomock"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/test/bufconn"
)

func TestWithBandwidthLimitedContextDialer(t *testing.T) {
	log.SetFlags(log.LstdFlags | log.Lmicroseconds)
	ctx := context.Background()
	ctrl := gomock.NewController(t)
	srv := grpc.NewServer()

	// use in-memory connection
	lis := bufconn.Listen(1024 * 1024)
	dialer := func(ctx context.Context, _ string) (net.Conn, error) {
		return lis.DialContext(ctx)
	}

	// create and register simple mock server
	mockServer := testproto.NewMockTestServiceServer(ctrl)
	mockServer.EXPECT().TestRPC(gomock.Any(), gomock.Any()).DoAndReturn(func(ctx context.Context, request *testproto.TestRequest) (*testproto.TestResponse, error) {
		return &testproto.TestResponse{Code: 1}, nil
	})
	testproto.RegisterTestServiceServer(srv, mockServer)

	// start gRPC server
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ll := bwlimit.NewListener(lis, 0, 10)
		if err := srv.Serve(ll); err != nil {
			log.Fatalf("Server exited with error: %v", err)
		}
	}()
	defer func() {
		srv.Stop()
		wg.Wait()
	}()

	// open rate limited client connection, limited to 10 B/s
	conn, err := grpc.DialContext(ctx,
		"bufnet",
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		// this interceptor limits the bandwith
		WithBandwidthLimitedContextDialer(0, 0, dialer),
	)
	if err != nil {
		t.Fatalf("Failed to dial bufnet: %v", err)
	}
	defer conn.Close()

	// create gRPC service client and measure how long it takes to get a response
	c := testproto.NewTestServiceClient(conn)
	before := time.Now()
	resp, err := c.TestRPC(ctx, &testproto.TestRequest{Id: "abcdefghijklmnopqrstuvwxyz"})
	elapsed := time.Since(before)
	if err != nil {
		t.Fatalf("Failed to call TestRPC: %v", err)
	}

	t.Log(resp)
	t.Log(elapsed) // it takes ~15 seconds, since we need to write 155 bytes and are rate limited to 10 B/s
}
