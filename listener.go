package bwlimit

import "net"

type Listener struct {
	net.Listener

	writeBytesPerSecond Byte
	readBytesPerSecond  Byte
}

func NewListener(lis net.Listener, writeLimitPerSecond, readLimitPerSecond Byte) *Listener {
	bwlis := &Listener{Listener: lis}
	bwlis.SetWriteBandwidthLimit(writeLimitPerSecond)
	bwlis.SetReadBandwidthLimit(readLimitPerSecond)
	return bwlis
}

func (l *Listener) Accept() (net.Conn, error) {
	conn, err := l.Listener.Accept()
	if err != nil {
		return nil, err
	}
	return NewConn(conn, l.writeBytesPerSecond, l.readBytesPerSecond), nil
}

func (l *Listener) WriteBandwidthLimit() Byte {
	return l.writeBytesPerSecond
}

func (l *Listener) ReadBandwidthLimit() Byte {
	return l.readBytesPerSecond
}

func (l *Listener) SetWriteBandwidthLimit(bytesPerSecond Byte) {
	if bytesPerSecond <= 0 {
		l.writeBytesPerSecond = 0
		return
	}
	l.writeBytesPerSecond = bytesPerSecond
}

func (l *Listener) SetReadBandwidthLimit(bytesPerSecond Byte) {
	if bytesPerSecond <= 0 {
		l.readBytesPerSecond = 0
		return
	}
	l.readBytesPerSecond = bytesPerSecond
}
