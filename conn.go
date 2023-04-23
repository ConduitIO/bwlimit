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
	"net"
	"os"
	"time"

	"golang.org/x/time/rate"
)

func NewConn(conn net.Conn, writeLimitPerSecond, readLimitPerSecond Byte) *Conn {
	bwconn := &Conn{Conn: conn}
	bwconn.SetWriteBandwidthLimit(writeLimitPerSecond)
	bwconn.SetReadBandwidthLimit(readLimitPerSecond)
	return bwconn
}

// Conn is a net.Conn connection that limits the bandwidth of writes and reads.
type Conn struct {
	net.Conn
	writeLimiter *rate.Limiter
	readLimiter  *rate.Limiter

	writeDeadline time.Time
	readDeadline  time.Time
}

// Write writes data to the connection.
// Write can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetWriteDeadline.
// Write will rate limit the connection bandwidth if a limit is configured. If
// the size of b is bigger than the rate of bytes per second, writes will be
// chunked into smaller units.
func (c *Conn) Write(b []byte) (n int, err error) {
	if c.writeLimiter == nil {
		// no limit, just pass the call through to the connection
		return c.Conn.Write(b)
	}

	ctx := context.Background()
	if !c.writeDeadline.IsZero() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, c.writeDeadline)
		defer cancel()
	}

	for _, chunk := range c.split(b, int(c.writeLimiter.Limit())) {
		bytesWritten, err := c.writeWithRateLimit(ctx, chunk)
		n += bytesWritten
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

func (c *Conn) writeWithRateLimit(ctx context.Context, b []byte) (int, error) {
	err := c.writeLimiter.WaitN(ctx, len(b))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			// according to net.Conn the error should be os.ErrDeadlineExceeded
			err = os.ErrDeadlineExceeded
		}
		return 0, err
	}
	return c.Conn.Write(b)
}

// Read reads data from the connection.
// Read can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetReadDeadline.
// Read will rate limit the connection bandwidth if a limit is configured. If
// the size of b is bigger than the rate of bytes per second, reads will be
// chunked into smaller units.
// Note that since it's not known in advance how many bytes will be read, the
// bandwidth can burst up to 2x of the configured limit when reading the first 2
// chunks.
func (c *Conn) Read(b []byte) (n int, err error) {
	if c.readLimiter == nil {
		// no limit, just pass the call through to the connection
		return c.Conn.Read(b)
	}

	ctx := context.Background()
	if !c.readDeadline.IsZero() {
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, c.readDeadline)
		defer cancel()
	}

	for _, chunk := range c.split(b, int(c.readLimiter.Limit())) {
		bytesRead, err := c.readWithRateLimit(ctx, chunk)
		n += bytesRead
		if err != nil {
			return n, err
		}
		if bytesRead < len(b) {
			return n, nil // we read all there is
		}
	}
	return n, nil
}

func (c *Conn) readWithRateLimit(ctx context.Context, b []byte) (int, error) {
	// we don't know how many bytes will actually be read, so we optimistically
	// wait until we can read at least 1 byte, at the end we additionally
	// reserve the number of actually read bytes so future reads are delayed
	err := c.readLimiter.WaitN(ctx, 1)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			// according to net.Conn the error should be os.ErrDeadlineExceeded
			err = os.ErrDeadlineExceeded
		}
		return 0, err
	}

	n, err := c.Conn.Read(b)
	if n > 1 {
		// reserve the number of actually read bytes to delay future reads
		_ = c.readLimiter.ReserveN(time.Now(), n-1)
	}
	return n, err
}

// split takes a byte slice and splits it into chunks of size maxSize. If b is
// not divisible by maxSize, the last chunk will be smaller.
func (c *Conn) split(b []byte, maxSize int) [][]byte {
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

// SetDeadline sets the read and write deadlines associated
// with the connection. It is equivalent to calling both
// SetReadDeadline and SetWriteDeadline.
//
// A deadline is an absolute time after which I/O operations
// fail instead of blocking. The deadline applies to all future
// and pending I/O, not just the immediately following call to
// Read or Write. After a deadline has been exceeded, the
// connection can be refreshed by setting a deadline in the future.
//
// If the deadline is exceeded a call to Read or Write or to other
// I/O methods will return an error that wraps os.ErrDeadlineExceeded.
// This can be tested using errors.Is(err, os.ErrDeadlineExceeded).
// The error's Timeout method will return true, but note that there
// are other possible errors for which the Timeout method will
// return true even if the deadline has not been exceeded.
//
// An idle timeout can be implemented by repeatedly extending
// the deadline after successful Read or Write calls.
//
// A zero value for t means I/O operations will not time out.
func (c *Conn) SetDeadline(t time.Time) error {
	err := c.Conn.SetDeadline(t)
	if err == nil {
		c.writeDeadline = t
		c.readDeadline = t
	}
	return err
}

// SetWriteDeadline sets the deadline for future Write calls
// and any currently-blocked Write call.
// Even if write times out, it may return n > 0, indicating that
// some of the data was successfully written.
// A zero value for t means Write will not time out.
func (c *Conn) SetWriteDeadline(t time.Time) error {
	err := c.Conn.SetWriteDeadline(t)
	if err == nil {
		c.writeDeadline = t
	}
	return err
}

// SetReadDeadline sets the deadline for future Read calls
// and any currently-blocked Read call.
// A zero value for t means Read will not time out.
func (c *Conn) SetReadDeadline(t time.Time) error {
	err := c.Conn.SetReadDeadline(t)
	if err == nil {
		c.readDeadline = t
	}
	return err
}

func (c *Conn) SetWriteBandwidthLimit(bytesPerSecond Byte) {
	if bytesPerSecond <= 0 {
		c.writeLimiter = nil
		return
	}
	c.writeLimiter = rate.NewLimiter(rate.Limit(bytesPerSecond), int(bytesPerSecond))
}

func (c *Conn) SetReadBandwidthLimit(bytesPerSecond Byte) {
	if bytesPerSecond <= 0 {
		c.readLimiter = nil
		return
	}
	c.readLimiter = rate.NewLimiter(rate.Limit(bytesPerSecond), int(bytesPerSecond))
}

func (c *Conn) SetBandwidthLimit(bytesPerSecond Byte) {
	if bytesPerSecond <= 0 {
		c.writeLimiter = nil
		c.readLimiter = nil
		return
	}
	c.writeLimiter = rate.NewLimiter(rate.Limit(bytesPerSecond), int(bytesPerSecond))
	c.readLimiter = rate.NewLimiter(rate.Limit(bytesPerSecond), int(bytesPerSecond))
}

func (c *Conn) WriteBandwidthLimit() Byte {
	if c.writeLimiter == nil {
		return 0
	}
	return Byte(c.writeLimiter.Limit())
}

func (c *Conn) ReadBandwidthLimit() Byte {
	if c.readLimiter == nil {
		return 0
	}
	return Byte(c.readLimiter.Limit())
}
