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
	"errors"
	"io"
	"math"
	"os"
	"time"

	"golang.org/x/time/rate"
)

type Reader struct {
	io.Reader

	limiter  *rate.Limiter
	deadline time.Time
}

func NewReader(r io.Reader, bytesPerSecond Byte) *Reader {
	bwr := &Reader{
		Reader:  r,
		limiter: rate.NewLimiter(toLimit(bytesPerSecond), toBurst(bytesPerSecond)),
	}
	return bwr
}

func (r *Reader) Deadline() time.Time {
	return r.deadline
}

func (r *Reader) SetDeadline(t time.Time) {
	r.deadline = t
}

func (r *Reader) BandwidthLimit() Byte {
	return Byte(r.limiter.Limit())
}

func (r *Reader) SetBandwidthLimit(bytesPerSecond Byte) {
	r.limiter.SetLimit(toLimit(bytesPerSecond))
	r.limiter.SetBurst(toBurst(bytesPerSecond))
}

// Close forwards the call to the wrapped io.Reader if it implements io.Closer,
// otherwise it is a noop.
func (r *Reader) Close() error {
	if rc, ok := r.Reader.(io.Closer); ok {
		return rc.Close()
	}
	return nil
}

func (r *Reader) Read(p []byte) (n int, err error) {
	if r.limiter.Limit() == rate.Inf {
		// no limit, just pass the call through to the connection
		return r.Reader.Read(p)
	}

	ctx := context.Background()
	if !r.deadline.IsZero() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, r.deadline)
		defer cancel()
	}

	for _, chunk := range split(p, int(r.limiter.Limit())) {
		bytesRead, err := r.readWithRateLimit(ctx, chunk)
		n += bytesRead
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

func (r *Reader) readWithRateLimit(ctx context.Context, p []byte) (int, error) {
	// we don't know how many bytes will actually be read, so we optimistically
	// wait until we can read at least 1 byte, at the end we additionally
	// reserve the number of actually read bytes so future reads are delayed
	err := r.limiter.WaitN(ctx, 1)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			// according to net.Conn the error should be os.ErrDeadlineExceeded
			err = os.ErrDeadlineExceeded
		}
		return 0, err
	}

	at := time.Now()
	n, err := r.Reader.Read(p)
	if n > 1 {
		// reserve the number of actually read bytes to delay future reads
		_ = r.limiter.ReserveN(at, n-1)
	}
	return n, err
}

type Writer struct {
	io.Writer

	limiter  *rate.Limiter
	deadline time.Time
}

func NewWriter(w io.Writer, bytesPerSecond Byte) *Writer {
	bwr := &Writer{
		Writer:  w,
		limiter: rate.NewLimiter(toLimit(bytesPerSecond), toBurst(bytesPerSecond)),
	}
	return bwr
}

func (w *Writer) Deadline() time.Time {
	return w.deadline
}

func (w *Writer) SetDeadline(t time.Time) {
	w.deadline = t
}

func (w *Writer) BandwidthLimit() Byte {
	return Byte(w.limiter.Limit())
}

func (w *Writer) SetBandwidthLimit(bytesPerSecond Byte) {
	w.limiter.SetLimit(toLimit(bytesPerSecond))
	w.limiter.SetBurst(toBurst(bytesPerSecond))
}

// Close forwards the call to the wrapped io.Reader if it implements io.Closer,
// otherwise it is a noop.
func (w *Writer) Close() error {
	if wc, ok := w.Writer.(io.Closer); ok {
		return wc.Close()
	}
	return nil
}

func (w *Writer) Write(p []byte) (n int, err error) {
	if w.limiter.Limit() == rate.Inf {
		// no limit, just pass the call through to the connection
		return w.Writer.Write(p)
	}

	ctx := context.Background()
	if !w.deadline.IsZero() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, w.deadline)
		defer cancel()
	}

	for _, chunk := range split(p, int(w.limiter.Limit())) {
		bytesWritten, err := w.writeWithRateLimit(ctx, chunk)
		n += bytesWritten
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

func (w *Writer) writeWithRateLimit(ctx context.Context, b []byte) (int, error) {
	err := w.limiter.WaitN(ctx, len(b))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			// according to net.Conn the error should be os.ErrDeadlineExceeded
			err = os.ErrDeadlineExceeded
		}
		return 0, err
	}
	return w.Writer.Write(b)
}

// split takes a byte slice and splits it into chunks of size maxSize. If b is
// not divisible by maxSize, the last chunk will be smaller.
func split(b []byte, maxSize int) [][]byte {
	var end int
	out := make([][]byte, ((len(b)-1)/maxSize)+1)
	for i := range out {
		start := end
		end += maxSize
		if end > len(b) {
			end = len(b)
		}
		out[i] = b[start:end]
	}
	return out
}

func toLimit(b Byte) rate.Limit {
	if b <= 0 {
		return rate.Inf
	}
	return rate.Limit(b)
}

func toBurst(b Byte) int {
	if b <= 0 {
		return math.MaxInt
	}
	return int(b)
}
