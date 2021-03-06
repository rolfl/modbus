package modbus

import (
	"net"
)

// NewTCP establishes a connection to a remote IP and port using TCP then returns a Modbus instance on that TCP channel
// using NewTCPConn(connection)
//
// e.g. NewTCP("192.168.1.10:502")
func NewTCP(hostport string) (Modbus, error) {
	addr, err := net.ResolveTCPAddr("tcp", hostport)
	if err != nil {
		return nil, err
	}

	// dial from any local interface to the remote address
	conn, err := net.DialTCP("tcp", nil, addr)
	if err != nil {
		return nil, err
	}

	return NewTCPConn(conn)
}
