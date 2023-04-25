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
	if conn != nil {
		conn = NewConn(conn, l.writeBytesPerSecond, l.readBytesPerSecond)
	}
	return conn, err
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
