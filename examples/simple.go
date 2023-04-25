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

	resp, _ := http.DefaultClient.Get("http://localhost:8080/echo")
	_, _ = io.ReadAll(resp.Body)
}
