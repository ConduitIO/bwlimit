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
	// we don't know how many bytes will actually be read so we optimistically
	// wait until we can read at least 1 byte, at the end we additionally
	// reserve the number of actually read bytes
	err := c.readLimiter.WaitN(ctx, 1)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			// according to net.Conn the error should be os.ErrDeadlineExceeded
			err = os.ErrDeadlineExceeded
		}
		return 0, err
	}

	n, err := c.Conn.Read(b)
	if n > 0 {
		// reserve the number of actually read bytes to delay future reads
		_ = c.readLimiter.ReserveN(time.Now(), n-1)
	}
	return n, err
}

func (c *Conn) split(b []byte, maxSize int) [][]byte {
	var end int
	out := make([][]byte, ((len(b)-1)/maxSize)+1)
	for i := range out {
		start := end
		end = end + maxSize
		if end > len(b) {
			end = len(b)
		}
		out[i] = b[start:end]
	}
	return out
}

func (c *Conn) SetDeadline(t time.Time) error {
	err := c.Conn.SetDeadline(t)
	if err == nil {
		c.writeDeadline = t
		c.readDeadline = t
	}
	return err
}

func (c *Conn) SetWriteDeadline(t time.Time) error {
	err := c.Conn.SetWriteDeadline(t)
	if err == nil {
		c.writeDeadline = t
	}
	return err
}

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
