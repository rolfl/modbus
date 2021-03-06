package modbus

import (
	"fmt"
	"io"
	"net"
)

// TCPServer represents a mechanism for receiving connections from remote clients.
// Note that this is not a Modbus server, but a TCP service, ready to accept connections
// and from each connection create a Modbus instance using NewTCPConn(...)
type TCPServer interface {
	io.Closer
	// WaitClosed will simply wait until the TCP server is closed. This is useful for creating
	// programs that don't exit until the listener is terminated.
	WaitClosed()
}

type tcpServer struct {
	tcpl    *net.TCPListener
	host    string
	servers map[byte]Server
	closed  chan bool
}

// ServeAllUnits is a convenience function to map a Modbus Server instance on to all unitID addresses.
func ServeAllUnits(server Server) map[int]Server {
	ret := make(map[int]Server)
	ret[0xFF] = server
	return ret
}

/*
NewTCPServer establishes a listening socket to accept incoming TCP requests. Use ":{port}" style value to bind
to all interfaces on the host. Use a specific local IP or local hostname to bind to just one interface.

Example bind to all interfaces: NewModbusTCPListener(":502", demux)

Example bind to just localhost: NewModbusTCPListener("localhost:502", demux)

Note that this function accepts a UnitID to Server mapping. Any connections to this server will be initialized
with the supplied servers serving requests to the matching UnitID. It's normal for Modbus-TCP to have 1 server
instance hosting ALL the UnitID addresses on the bus. The standard is to listen on UnitID 0xff. This is made
more convenient with the ServeAllUnits(server) function.const

	tcpserv, _ := modbus.NewTCPServer(":502", modbus.ServeAllUnits(server))

*/
func NewTCPServer(host string, servers map[int]Server) (TCPServer, error) {
	laddr, err := net.ResolveTCPAddr("tcp", host)
	if err != nil {
		return nil, err
	}
	tcpl, err := net.ListenTCP("tcp", laddr)
	if err != nil {
		return nil, err
	}
	mservers := make(map[byte]Server)
	for u, s := range servers {
		mservers[bytePanic(u)] = s
	}
	tlistener := &tcpServer{tcpl, host, mservers, make(chan bool)}
	go tlistener.monitor()
	return tlistener, nil
}

func (t *tcpServer) Close() error {
	return t.tcpl.Close()
}

func (t *tcpServer) WaitClosed() {
	<-t.closed
}

func (t *tcpServer) monitor() {
	// defer tcpl.Close()
	for {
		conn, err := t.tcpl.AcceptTCP()
		if err != nil {
			fmt.Printf("Error awaiting connections on %v: %v\n", t.host, err)
			close(t.closed)
			break
		}
		m, err := NewTCPConn(conn)
		if err != nil {
			fmt.Printf("Error establishing Modbus connection from remote %v to local %v: %v\n", conn.RemoteAddr(), t.host, err)
		} else {
			for u, s := range t.servers {
				m.SetServer(int(u), s)
			}
		}
	}
}
