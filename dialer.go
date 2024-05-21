package adb

import (
	"fmt"
	"net"

	"github.com/prife/goadb/wire"
)

// Dialer knows how to create connections to an adb server.
type Dialer interface {
	Dial(address string) (*wire.Conn, error)
}

type tcpDialer struct{}

// Dial connects to the adb server on the host and port set on the netDialer.
// The zero-value will connect to the default, localhost:5037.
func (tcpDialer) Dial(address string) (*wire.Conn, error) {
	netConn, err := net.Dial("tcp", address)
	if err != nil {
		return nil, fmt.Errorf("%w: error dialing %s", wire.ErrServerNotAvailable, address)
	}

	return wire.NewConn(netConn), nil
}
