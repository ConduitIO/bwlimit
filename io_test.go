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
	"crypto/rand"
	"fmt"
	"io"
	"testing"
	"time"
)

// epsilon is the allowed difference between the expected and actual delay
var epsilon time.Duration

func init() {
	// we measure how long it takes to read 10 bytes and calculate the
	// acceptable epsilon in read times when limiting the bandwidth
	r := randomReader(10)
	start := time.Now()
	_, err := r.Read(make([]byte, 2))
	epsilon = time.Millisecond + (time.Since(start) * 20)
	if err != nil {
		panic(fmt.Errorf("error reading: %v", err))
	}
}

func randomReader(size int64) io.Reader {
	pr, pw := io.Pipe()
	go func() {
		random := io.LimitReader(rand.Reader, size)
		_, err := io.Copy(pw, random)
		_ = pw.CloseWithError(err)
	}()
	return pr
}

func TestReader_Read(t *testing.T) {
	t.Logf("epsilon: %v", epsilon)

	testCases := []struct {
		byteCount      int64
		bytesPerSecond Byte
		wantDelay      time.Duration
	}{{
		byteCount:      1,
		bytesPerSecond: 0,
		wantDelay:      0, // no delay
	}, {
		byteCount:      1,
		bytesPerSecond: 1,
		wantDelay:      0, // no delay
	}, {
		byteCount:      4,
		bytesPerSecond: 2,
		// expected delay is 1 second:
		// - first read happens instantly, returns 2 bytes
		// - second read happens instantly (reservation contains no delay), returns 2 bytes
		// - third read is delayed by 1s (reservation contains delay), returns EOF
		wantDelay: 1 * time.Second,
	}, {
		byteCount:      102,
		bytesPerSecond: 100,
		// expected delay is 20 milliseconds:
		// - first read happens instantly, returns 100 bytes
		// - second read happens instantly (reservation contains no delay), returns 2 bytes
		// - third read is delayed by 20ms (2/100 of a second, as reservation contains 2 tokens), returns EOF
		wantDelay: 20 * time.Millisecond,
	}, {
		byteCount:      103,
		bytesPerSecond: 100,
		// expected delay is 30 milliseconds:
		// - first read happens instantly, returns 100 bytes
		// - second read happens instantly (reservation contains no delay), returns 3 bytes
		// - third read is delayed by 30ms (3/100 of a second, as reservation contains 3 tokens), returns EOF
		wantDelay: 30 * time.Millisecond,
	}}

	for _, tc := range testCases {
		tc := tc // assign to local var as test cases run in parallel
		t.Run(fmt.Sprintf("%d byte/%d bps", tc.byteCount, tc.bytesPerSecond), func(t *testing.T) {
			// these tests are blocking, run them in parallel
			t.Parallel()

			r := NewReader(randomReader(tc.byteCount), tc.bytesPerSecond)
			start := time.Now()
			got, err := io.ReadAll(r)
			gotDelay := time.Since(start)
			if err != nil {
				t.Fatalf("error reading: %v", err)
			}

			if len(got) != int(tc.byteCount) {
				t.Fatalf("expected %d bytes, got %d", tc.byteCount, len(got))
			}

			gotDiff := tc.wantDelay - gotDelay
			if gotDiff < 0 {
				gotDiff = -gotDiff
			}

			if gotDiff > epsilon {
				t.Fatalf("expected a maximum delay of %v, got %v, allowed epsilon of %v exceeded", tc.wantDelay, gotDelay, epsilon)
			}
		})
	}
}

func TestWriter_Write(t *testing.T) {
	t.Logf("epsilon: %v", epsilon)

	testCases := []struct {
		byteCount      int64
		bytesPerSecond Byte
		wantDelay      time.Duration
	}{{
		byteCount:      1,
		bytesPerSecond: 0,
		wantDelay:      0, // no delay
	}, {
		byteCount:      1,
		bytesPerSecond: 1,
		wantDelay:      0, // no delay
	}, {
		byteCount:      4,
		bytesPerSecond: 2,
		// expected delay is 1 second:
		// - first write happens instantly, writes 2 bytes
		// - second write is delayed by 1s, writes 2 bytes
		wantDelay: 1 * time.Second,
	}, {
		byteCount:      102,
		bytesPerSecond: 100,
		// expected delay is 20 milliseconds:
		// - first write happens instantly, writes 100 bytes
		// - second write is delayed by 20ms (2/100 of a second to reserve 2 tokens), writes 2 bytes
		wantDelay: 20 * time.Millisecond,
	}, {
		byteCount:      103,
		bytesPerSecond: 100,
		// expected delay is 30 milliseconds:
		// - first write happens instantly, writes 100 bytes
		// - second write is delayed by 30ms (3/100 of a second to reserve 3 tokens), writes 3 bytes
		wantDelay: 30 * time.Millisecond,
	}}

	for _, tc := range testCases {
		tc := tc // assign to local var as test cases run in parallel
		t.Run(fmt.Sprintf("%d byte/%d bps", tc.byteCount, tc.bytesPerSecond), func(t *testing.T) {
			// these tests are blocking, run them in parallel
			t.Parallel()

			w := NewWriter(io.Discard, tc.bytesPerSecond)
			start := time.Now()
			n, err := io.Copy(w, randomReader(tc.byteCount))
			gotDelay := time.Since(start)
			if err != nil {
				t.Fatalf("error reading: %v", err)
			}

			if n != tc.byteCount {
				t.Fatalf("expected %d bytes, got %d", tc.byteCount, n)
			}

			gotDiff := tc.wantDelay - gotDelay
			if gotDiff < 0 {
				gotDiff = -gotDiff
			}

			if gotDiff > epsilon {
				t.Fatalf("expected a maximum delay of %v, got %v, allowed epsilon of %v exceeded", tc.wantDelay, gotDelay, epsilon)
			}
		})
	}
}
