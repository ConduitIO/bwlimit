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
	"net/http"

	"golang.org/x/time/rate"
)

type RoundTripper struct {
	http.RoundTripper

	writeLimiter *rate.Limiter
	readLimiter  *rate.Limiter
}

func NewRoundTripper(rt http.RoundTripper, writeLimitPerSecond, readLimitPerSecond Byte) http.RoundTripper {
	bwrt := &RoundTripper{
		RoundTripper: rt,
		writeLimiter: &rate.Limiter{},
		readLimiter:  &rate.Limiter{},
	}
	bwrt.SetWriteBandwidthLimit(writeLimitPerSecond)
	bwrt.SetReadBandwidthLimit(readLimitPerSecond)
	return bwrt
}

func (rt *RoundTripper) RoundTrip(r *http.Request) (*http.Response, error) {
	r.Body = &Reader{
		Reader:  r.Body,
		limiter: rt.writeLimiter, // share limiter across all requests
	}
	resp, err := rt.RoundTripper.RoundTrip(r)
	if resp != nil {
		resp.Body = &Reader{
			Reader:  resp.Body,
			limiter: rt.readLimiter, // share limiter across all responses
		}
	}
	return resp, err
}

func (rt *RoundTripper) SetWriteBandwidthLimit(bytesPerSecond Byte) {
	if bytesPerSecond <= 0 {
		rt.writeLimiter.SetLimit(rate.Inf)
		return
	}
	rt.writeLimiter.SetLimit(rate.Limit(bytesPerSecond))
	rt.writeLimiter.SetBurst(int(bytesPerSecond))
}

func (rt *RoundTripper) SetReadBandwidthLimit(bytesPerSecond Byte) {
	if bytesPerSecond <= 0 {
		rt.readLimiter.SetLimit(rate.Inf)
		return
	}
	rt.readLimiter.SetLimit(rate.Limit(bytesPerSecond))
	rt.readLimiter.SetBurst(int(bytesPerSecond))
}
