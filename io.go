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

// Reader wraps an io.Reader and imposes a bandwidth limit on calls to Read.
type Reader struct {
	io.Reader

	limiter     *rate.Limiter
	reservation *rate.Reservation
	deadline    time.Time
}

// NewReader wraps an existing io.Reader and returns a Reader that limits the
// bandwidth of reads.
// A zero value for bytesPerSecond means that Read will not have a bandwidth
// limit.
func NewReader(r io.Reader, bytesPerSecond Byte) *Reader {
	bwr := &Reader{
		Reader:  r,
		limiter: rate.NewLimiter(toLimit(bytesPerSecond), toBurst(bytesPerSecond)),
	}
	return bwr
}

// Deadline returns the configured deadline (see SetDeadline).
func (r *Reader) Deadline() time.Time {
	return r.deadline
}

// SetDeadline sets the read deadline associated with the reader.
//
// A deadline is an absolute time after which Read fails instead of blocking.
// The deadline applies to all future and pending calls to Read, not just the
// immediately following call to Read. After a deadline has been exceeded, the
// reader can be refreshed by setting a deadline in the future.
//
// If the deadline is exceeded a call to Read will return an error that wraps
// os.ErrDeadlineExceeded. This can be tested using
// errors.Is(err, os.ErrDeadlineExceeded).
//
// An idle timeout can be implemented by repeatedly extending the deadline after
// successful Read calls.
//
// A zero value for t means that calls to Read will not time out.
func (r *Reader) SetDeadline(t time.Time) {
	r.deadline = t
}

// BandwidthLimit returns the current bandwidth limit.
func (r *Reader) BandwidthLimit() Byte {
	return Byte(r.limiter.Limit())
}

// SetBandwidthLimit sets the bandwidth limit for future Read calls and
// any currently-blocked Read call.
// A zero value for bytesPerSecond means the bandwidth limit is removed.
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

// Read reads up to len(p) bytes into p. It returns the number of bytes
// read (0 <= n <= len(p)) and any error encountered.
//
// Read will limit the speed of the reads if a bandwidth limit is configured. If
// the size of p is bigger than the rate of bytes per second, reads will be
// split into smaller chunks.
// Note that since it's not known in advance how many bytes will be read, the
// bandwidth can burst up to 2x of the configured limit when reading the first 2
// chunks.
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
		if bytesRead < len(p) {
			// we did not read a whole chunk, we need to return and let the
			// caller call Read again, so we split the bytes again in new chunks
			return n, nil
		}
	}
	return n, nil
}

// readWithRateLimit will delay the read if needed to match the configured
// bandwidth limit.
func (r *Reader) readWithRateLimit(ctx context.Context, p []byte) (int, error) {
	// we first need to delay the read if there was a read that happened before
	if r.reservation != nil && r.reservation.OK() {
		err := r.wait(ctx, r.reservation.Delay())
		if err != nil {
			if errors.Is(err, context.DeadlineExceeded) {
				// according to net.Conn the error should be os.ErrDeadlineExceeded
				err = os.ErrDeadlineExceeded
			}
			return 0, err
		}
		r.reservation = nil
	}

	at := time.Now()
	n, err := r.Reader.Read(p)
	if n > 0 {
		// reserve the number of actually read bytes to delay future reads
		r.reservation = r.limiter.ReserveN(at, n)
	}
	return n, err
}

func (r *Reader) wait(ctx context.Context, d time.Duration) error {
	if d == 0 {
		return nil
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-timer.C:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// Writer wraps an io.Writer and imposes a bandwidth limit on calls to Write.
type Writer struct {
	io.Writer

	limiter  *rate.Limiter
	deadline time.Time
}

// NewWriter wraps an existing io.Writer and returns a Writer that limits the
// bandwidth of writes.
// A zero value for bytesPerSecond means that Write will not have a bandwidth
// limit.
func NewWriter(w io.Writer, bytesPerSecond Byte) *Writer {
	bwr := &Writer{
		Writer:  w,
		limiter: rate.NewLimiter(toLimit(bytesPerSecond), toBurst(bytesPerSecond)),
	}
	return bwr
}

// Deadline returns the configured deadline (see SetDeadline).
func (w *Writer) Deadline() time.Time {
	return w.deadline
}

// SetDeadline sets the write deadline associated with the writer.
//
// A deadline is an absolute time after which Write fails instead of blocking.
// The deadline applies to all future and pending calls to Write, not just the
// immediately following call to Write. After a deadline has been exceeded, the
// writer can be refreshed by setting a deadline in the future.
//
// If the deadline is exceeded a call to Write will return an error that wraps
// os.ErrDeadlineExceeded. This can be tested using
// errors.Is(err, os.ErrDeadlineExceeded).
//
// An idle timeout can be implemented by repeatedly extending the deadline after
// successful Write calls.
//
// A zero value for t means that calls to Write will not time out.
func (w *Writer) SetDeadline(t time.Time) {
	w.deadline = t
}

// BandwidthLimit returns the current bandwidth limit.
func (w *Writer) BandwidthLimit() Byte {
	return Byte(w.limiter.Limit())
}

// SetBandwidthLimit sets the bandwidth limit for future Write calls and
// any currently-blocked Write call.
// A zero value for bytesPerSecond means the bandwidth limit is removed.
func (w *Writer) SetBandwidthLimit(bytesPerSecond Byte) {
	w.limiter.SetLimit(toLimit(bytesPerSecond))
	w.limiter.SetBurst(toBurst(bytesPerSecond))
}

// Close forwards the call to the wrapped io.Writer if it implements io.Closer,
// otherwise it is a noop.
func (w *Writer) Close() error {
	if wc, ok := w.Writer.(io.Closer); ok {
		return wc.Close()
	}
	return nil
}

// Write writes len(p) bytes from p to the underlying data stream.
// It returns the number of bytes written from p (0 <= n <= len(p))
// and any error encountered that caused the write to stop early.
//
// Write will limit the speed of the writes if a bandwidth limit is configured.
// If the size of p is bigger than the rate of bytes per second, writes will be
// split into smaller chunks.
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

// writeWithRateLimit will delay the write if needed to match the configured
// bandwidth limit.
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
