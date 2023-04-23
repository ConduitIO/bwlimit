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

package main

import (
	"crypto/rand"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/conduitio/bwlimit"
)

const (
	addr        = "http://localhost:8080/echo"
	requestSize = 20 * bwlimit.KB

	readLimit  = 4 * bwlimit.KB // read limit is 4000 B/s
	writeLimit = bwlimit.MiB    // write limit is 1048576 B/s
)

func main() {
	// change default transport to use bandwidth limit
	t := (http.DefaultTransport).(*http.Transport)
	bwlimit.TransportWithBandwidthLimit(t, writeLimit, readLimit)

	pr, pw := io.Pipe()
	go func() {
		random := io.LimitReader(rand.Reader, int64(requestSize))
		_, err := io.Copy(pw, random)
		if err != nil {
			pw.CloseWithError(err)
		} else {
			pw.Close()
		}
	}()

	resp, err := send(pr)
	if err != nil {
		log.Fatalf("failed to send request: %v", err)
	}

	err = read(resp)
	if err != nil {
		log.Fatalf("failed to read response: %v", err)
	}
}

func send(body io.Reader) (*http.Response, error) {
	req, err := http.NewRequest("POST", addr, body)
	if err != nil {
		return nil, err
	}

	bandwidth := measureBandwidth("send")
	resp, err := http.DefaultClient.Do(req)
	bandwidth(int(requestSize))
	return resp, err
}

func read(resp *http.Response) error {
	bandwidth := measureBandwidth("read")
	body, err := io.ReadAll(resp.Body)
	bandwidth(len(body))
	return err
}

func measureBandwidth(operation string) func(count int) {
	start := time.Now()
	return func(count int) {
		elapsed := time.Since(start)
		log.Printf("%v bandwidth: %.0f B/s", operation, float64(count)/elapsed.Seconds())
	}
}
