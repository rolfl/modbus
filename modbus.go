/*
Package modbus provides interfaces to Modbus Clients and Servers via either TCP or RTU protocols.

Whether behaving as a Client or a Server, you need to establish a communication channel, either socket or serial based.
The Modbus protocol is established on top of the communication channel. Once a Modbus instance is created, you can
establish Client or Server instances on top of it. A client instance allows communication to a specific remote Server

Special note about Client instances: the Modbus documentation indicates that a "client" can talk to any of the servers
on the modbus, but this code requires that for each remote server has a unique Client instance to
communicate with it.

Establishing a Modbus communicationc channel using TCP is simple:

    mb, _ := modbus.NewTCP("host.example.com:502")

The above establishes a TCP connection on the standard port 502. It is normal, but not required, for the system initiating
the TCP to be the client. As a result, it would be normal if you wanted to communicate with the server at the unitID 5 to
follow the above line with:

    client := mb.GetClient(5)

With a client, you can perform all the standard Modbus functions against that server, for example, read 4 coils from
address 0 with a timeout of 2 seconds:

    coils, _ := client.ReadCoils(0, 4, time.Second*2)
	fmt.Printf("The 4 coils are %v\n", coils)

Similar to TCP, establishing an RTU Modbus instance is relatively simple, though additional data is required:

    mb, _ := modbus.NewRTU("COM5", 9600, 'E', 1, true)

The above establishes a serial communication channel on the serial port COM5 (windows) with 9600 baud, even parity, 1 stop
bit and it also sets the serial DTR line (some systems, espeically USB-based serial protocol converters need this).

The `mb` Modbus instance returned from the NewRTU function behaves the same way as the `mb` returned from NewTCP. You
can establish either/both a client presence or server presence on the Modbus. In this example we create a server at the
UnitID of 5. Servers are more complicated than clients - we need to establish a pattern of behaviour that the server
supports, including ID properties and more.

	deviceInfo := []string{"My VendorName", "My ProductCode", "My MajorMinor", "My VendorURL", "My ProductName", ....}
	server := []byte("MyServer ID")
	server, _ := modbus.NewServer(serverId, deviceInfo)

	mb.SetServer(5, server)

The above will establish a server that has no discretes, coils, inputs, registers, or files, but all the metadata and
counter logic will still work fine. You need to register handlers for coils, etc in order for them to be functional: more
detail about handlers is given in the Server documentaion.

The Modbus protocol relies heavily on 8-bit byte and 16-bit word values to communicate data. This library abstracts all the
type conversion and relies on basic Go `int` values instead. Where converting to the valid Modbus type is not possible due
to out-of-range values, a panic will be generated. The trade off for code complexity is significant. The public interface
for all modbus operations is thus completely int and bool based. The only exception is the byte-array for serverIDs.
*/
package modbus

import (
	"errors"
	"fmt"
)

type rtuFrame []byte

// pdu is the function and data sent on the Modbus.
type pdu struct {
	function byte
	data     rtuFrame
}

// adu is the data packet used to move Modbus data from a client to a specific server, and the response it gives.
type adu struct {
	request bool
	txid    uint16
	unit    byte
	pdu     pdu
}

type busErrorFunc func() int

/*
Modbus is a half duplex (or possibly full duplex) mechanism for talking to remote units.

Both Modbus TCP and RTU can be described this way. In order to create a Modbus instance you need to initialize
it using either the `modbus.NewTCPConn` or `modbus.NewRTU` constructors.

The Modbus instance can be used to get clients, add servers, or close the communication channel. In addition
you can get the current diagnostic state of the channel.
*/
type Modbus interface {
	//GetClient creates a control instance for communicating with a specific server on the remote side of the Modbus
	GetClient(unitID int) Client
	// SetServer establishes a server instance on the given unitId
	SetServer(unitID int, server Server)
	// Close closes the communication channel under the Modbus protocol
	Close() error
	// Diagnostics returns the current diagnostic counters for the Modbus channel
	Diagnostics() BusDiagnostics

	getEventLog() []int
	clearDiagnostics()
	clearOverrunCounter()
}

type modbus struct {
	tx      chan adu
	rx      chan adu
	clients map[byte]*client
	servers map[byte]Server
	pending map[uint16]bool
	closer  func() error
	txid    uint16
	diag    *busDiagnosticManager
}

func newModbus(tx chan adu, rx chan adu, closer func() error, diag *busDiagnosticManager) Modbus {
	mytx := make(chan adu, 0)
	m := &modbus{mytx, rx, make(map[byte]*client), make(map[byte]Server), make(map[uint16]bool), closer, 0, diag}
	go m.demuxRX()
	go m.associate(tx)
	return m
}

func (m *modbus) Close() error {
	return m.closer()
}

func (m *modbus) Diagnostics() BusDiagnostics {
	return m.diag.getDiagnostics()
}

func (m *modbus) getEventLog() []int {
	return m.diag.getEventLog()
}

func (m *modbus) clearDiagnostics() {
	m.diag.clear()
}

func (m *modbus) clearOverrunCounter() {
	m.diag.clearOverrun()
}

// GetClient estabishes a client that talks to a remote unit.
func (m *modbus) GetClient(unitID int) Client {
	unit := bytePanic(unitID)
	c := m.clients[unit]
	if c != nil {
		return c
	}
	// make a new one.
	c = &client{unit, m, make(chan pdu, 5)}
	m.clients[unit] = c
	return c
}

// SetServer sets a handler for when remote units talk to us.
func (m *modbus) SetServer(unit int, server Server) {
	m.servers[bytePanic(unit)] = server
}

func (m *modbus) associate(to chan adu) {
	for a := range m.tx {
		if a.request {
			m.pending[a.txid] = true
		}
		to <- a
	}
}

func (m *modbus) demuxRX() {
	for adu := range m.rx {
		if m.pending[adu.txid] {
			delete(m.pending, adu.txid)
			m.clients[adu.unit].rx <- adu.pdu
		} else if m.servers[adu.unit] != nil || m.servers[0xff] != nil {
			go m.handleServer(adu)
		} else if m.clients[adu.unit] != nil {
			fmt.Printf("Received packet for %v but that client is not expecting a response.\n", adu.unit)
		} else {
			fmt.Printf("Received packet for %v but there is nothing serving that address.\n", adu.unit)
		}
	}
}

func (m *modbus) handleServer(req adu) {
	server := m.servers[req.unit]
	if server == nil {
		server = m.servers[0xff]
	}
	data, err := server.request(m, req.unit, req.pdu.function, req.pdu.data)
	if err != nil {
		var mError *Error
		if !errors.As(err, &mError) {
			mError = ServerFailureErrorF("%v", err)
		}
		fmt.Printf("Request failed unit 0x%02x function 0x%02x: %v\n", req.unit, req.pdu.function, mError)
		p := mError.asPDU(req.pdu.function)
		rep := adu{false, req.txid, req.unit, p}
		m.tx <- rep
	} else {
		fmt.Printf("Handled unit 0x%02x function 0x%02x\n", req.unit, req.pdu.function)
		p := pdu{req.pdu.function, data}
		rep := adu{false, req.txid, req.unit, p}
		m.tx <- rep
	}
}
