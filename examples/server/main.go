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
	"io"
	"log"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/conduitio/bwlimit"
)

const (
	addr         = ":8080"
	echoEndpoint = "/echo"

	readLimit  = 0 // 4 * bwlimit.KB // read limit is 4000 B/s
	writeLimit = 0 // bwlimit.MiB    // write limit is 1048576 B/s
)

func main() {
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		log.Fatal(err)
	}
	// limit the listener bandwidth
	ln = bwlimit.NewListener(ln, writeLimit, readLimit)
	defer ln.Close()

	http.Handle(echoEndpoint, http.HandlerFunc(echoHandler))
	srv := &http.Server{Addr: addr}
	log.Fatal(srv.Serve(ln))
}

func echoHandler(w http.ResponseWriter, r *http.Request) {
	b, err := read(r)
	if err != nil {
		log.Printf("failed to read request: %v", err)
		return
	}

	err = write(w, b)
	if err != nil {
		log.Printf("failed to write response: %v", err)
	}
}

func read(r *http.Request) ([]byte, error) {
	defer r.Body.Close()
	bandwidth := measureBandwidth("read")
	body, err := io.ReadAll(r.Body)
	bandwidth(len(body))
	return body, err
}

func write(w http.ResponseWriter, b []byte) error {
	w.Header().Set("Content-Length", strconv.Itoa(len(b)))
	bandwidth := measureBandwidth("write")
	n, err := w.Write(b)
	bandwidth(n)
	return err
}

func measureBandwidth(operation string) func(count int) {
	start := time.Now()
	return func(count int) {
		elapsed := time.Since(start)
		log.Printf("%v bandwidth: %.0f B/s", operation, float64(count)/elapsed.Seconds())
	}
}
