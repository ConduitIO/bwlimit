package ratelimit

import (
	"context"
	"errors"
	"log"
	"net"
	"os"
	"time"

	"golang.org/x/time/rate"
)

func NewRateLimitedConn(conn net.Conn, writeBytesPerSecond int, readBytesPerSecond int) net.Conn {
	rconn := &rateLimitedConn{Conn: conn}
	if writeBytesPerSecond > 0 {
		rconn.writeLimiter = rate.NewLimiter(rate.Limit(writeBytesPerSecond), writeBytesPerSecond)
	}
	if readBytesPerSecond > 0 {
		rconn.readLimiter = rate.NewLimiter(rate.Limit(readBytesPerSecond), readBytesPerSecond)
	}
	return rconn
}

// rateLimitedConn is a net.Conn connection that limits the bandwith of writes
// and reads.
type rateLimitedConn struct {
	net.Conn
	writeLimiter *rate.Limiter
	readLimiter  *rate.Limiter

	writeDeadline time.Time
	readDeadline  time.Time
}

// Write writes data to the connection.
// Write can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetWriteDeadline.
// Write will rate limit the connection bandwidth if a writeLimiter is
// configured. If the size of b is bigger than the rate per second, writes will
// be chunked into smaller units.
func (c *rateLimitedConn) Write(b []byte) (n int, err error) {
	if c.writeLimiter == nil {
		// no limit, just pass the call through to the connection
		return c.Conn.Write(b)
	}

	ctx := context.Background()
	if !c.writeDeadline.IsZero() {
		log.Printf("setting deadline: %v", c.writeDeadline)
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, c.writeDeadline)
		defer cancel()
	}

	start := time.Now()
	defer func() {
		elapsed := time.Since(start)
		log.Printf("written %d bytes in %s, rate was %.2f B/s", n, elapsed, float64(n)/elapsed.Seconds())
		log.Println()
	}()
	for _, chunk := range c.split(b, int(c.writeLimiter.Limit())) {
		bytesWritten, err := c.writeWithRateLimit(ctx, chunk)
		n += bytesWritten
		if err != nil {
			return n, err
		}
	}
	return n, nil
}

func (c *rateLimitedConn) writeWithRateLimit(ctx context.Context, b []byte) (int, error) {
	log.Printf("waiting to write %d bytes", len(b))
	err := c.writeLimiter.WaitN(ctx, len(b))
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) {
			// according to net.Conn the error should be os.ErrDeadlineExceeded
			err = os.ErrDeadlineExceeded
		}
		return 0, err
	}
	log.Printf("writing %q...", b)
	return c.Conn.Write(b)
}

// Read reads data from the connection.
// Read can be made to time out and return an error after a fixed
// time limit; see SetDeadline and SetReadDeadline.
// Read will rate limit the connection bandwidth if a readLimiter is
// configured. If the size of b is bigger than the rate per second, reads will
// be chunked into smaller units.
func (c *rateLimitedConn) Read(b []byte) (n int, err error) {
	if c.readLimiter == nil {
		// no limit, just pass the call through to the connection
		return c.Conn.Read(b)
	}

	ctx := context.Background()
	if !c.readDeadline.IsZero() {
		log.Printf("setting deadline: %v", c.readDeadline)
		var cancel context.CancelFunc
		ctx, cancel = context.WithDeadline(ctx, c.readDeadline)
		defer cancel()
	}

	start := time.Now()
	defer func() {
		elapsed := time.Since(start)
		log.Printf("read %d bytes in %s, rate was %.2f B/s", n, elapsed, float64(n)/elapsed.Seconds())
		if err != nil {
			log.Printf("err: %v", err)
		}
		log.Println()
	}()
	log.Printf("will read up to %d bytes", len(b))
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

func (c *rateLimitedConn) readWithRateLimit(ctx context.Context, b []byte) (int, error) {
	log.Printf("waiting to read %d bytes", len(b))
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
	log.Printf("reading...")

	n, err := c.Conn.Read(b)
	if n > 0 {
		// reserve the number of actually read bytes to delay future reads
		_ = c.readLimiter.ReserveN(time.Now(), n-1)
	}
	return n, err
}

func (c *rateLimitedConn) split(b []byte, maxSize int) [][]byte {
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

func (c *rateLimitedConn) SetDeadline(t time.Time) error {
	err := c.Conn.SetDeadline(t)
	if err == nil {
		c.writeDeadline = t
		c.readDeadline = t
	}
	return err
}

func (c *rateLimitedConn) SetWriteDeadline(t time.Time) error {
	err := c.Conn.SetWriteDeadline(t)
	if err == nil {
		c.writeDeadline = t
	}
	return err
}

func (c *rateLimitedConn) SetReadDeadline(t time.Time) error {
	err := c.Conn.SetReadDeadline(t)
	if err == nil {
		c.readDeadline = t
	}
	return err
}
