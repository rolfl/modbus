# Modbus library - TCP/RTU and Client/Server interfaces

This code provides library access to Modbus devices, acting either as a client or as a server and supports RTU (serial) and TCP protocols (but not RTU-ASCII).

Note, the library supports access to all Modbus functions, and all but a few diagnostic sub-functions (cannot reset, or set to read-only when acting as a server).

As a client, you can call ALL functions on remote servers (discretes, coils, inputs, registers, files (including FIFO), and all diagnostic functions - counters, ids, logs, resets, etc.).

As a server, ALL data functions from clients are supported (discretes, coils, inputs, registers (including FIFO), files, and almost all diagnostoc functions - counters, ids, logs, counters & counter resets, but not device resets or read-only-mode)

# Design

This code separates the various layers of the Modbus protocols - it defines a "modbus" as a communication channel that can have client and server units connected. In the form of a RTU bus the specification requires at most 1 client, and allows many servers. In the form of a TCP connection there is typically only 1 client and 1 server. Not that the server may in fact be a proxy to other units, and that single connection could proxy many units.

Both RTU and TCP protocol modbus setups are exposed as a single `Modbbus` interface - once a `Modbus` instance exists, it becomes transparent as to whether the physical implementation is RTU or TCP.

Connecting your code to the Modbus is a simple case of getting a `Client` if you want to command a remote unit, or registering a `Server` if you want to allow remote clients to command your code.

## Notes about RTU

Establishing a serial connection requires pre-configured baud, parity, and stop-bit settings. In addition, the hardward-control DTR line may need to be set in order to initialize serial connections. The base modbus RTU specification indicates that 9600 baud, 8 bits, even parity, and 1 stop bit MUST be supported. This standard further suggests that support for "odd" and "no parity" should be implemented, but that when "no parity" support is used there should be 2 stop bits.

This library does not enforce any of these standard suggestions or requirements, it has no "default" settings, and as such "you" should ensure that the serial configuration is sane for a Modbus deployment.

### Example RTU Client

```go
mb, err := modbus.NewRTU("COM5", 9600, 'E', 1, true)
// ... error handling
client := mb.GetClient(5)

// ... do something with the client connection to remote UnitID 5
```

### Example RTU Server

```go
serialId := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
deviceInfo := []string{"ACME Corporation", "Widget Server", "v5.6.7", "http://github.com/rolfl/modbus"}
server, err := modbus.NewServer(serialId, deviceInfo)
// Register handlers for discretes, coils, inputs, registers & files

mb, err := modbus.NewRTU("COM5", 9600, 'E', 1, true)
// ... error handling
client := mb.SetServer(5, server)

// wait for the service to terminate

```

***Note:*** This library depends on tarm/serial to drive the serial port, but that library does not currently have DTR support. DTR is required to talk to a number of USB-to-serial transceivers. The plan is to contribute back the DTR (and possibly RTS) support back to tarm/serial, but it needs to work
on Linux first. For the moment, as per the tarm/serial license, the code has been copied in to this module, and modified to support DTR. See [tarm/serial](https://github.com/tarm/serial)

The reason `tarm/serial` is the best for this library is because:

- no cgo dependency (mikepb/go-serial) was considered, but compiling it is a real challenge
- it is well established (lots of eyes on it)
- it is relatively thin ..... not many added features
- it supports read timeouts

## Notes about TCP

In Modbus-TCP deployments the Server side is either:
- a regular Modbus server - handling function commands as expecter
- a "bridge" or "proxy" where it is a TCP frontend for multiple backend Modbus Servers

All Modbus servers are expected to have a unit ID between 1 and 247. If the remote system is a proxy/bridge then it will forward specific unit ids to the appropriate system behind the bridge. The TCP Guide states:

> This field is used for routing purpose when addressing a device on a MODBUS+ or MODBUS serial line sub-network. In that case, the “Unit Identifier” carries the MODBUS slave address of the remote device: If the MODBUS server is connected to a MODBUS+ or MODBUS Serial Line sub-network and addressed through a bridge or a gateway, the MODBUS Unit identifier is necessary to identify the slave device connected on the subnetwork behind the bridge or the gateway. The destination IP address identifies the bridge itself and the bridge uses the MODBUS Unit identifier to forward the request to the right slave device. The MODBUS slave device addresses on serial line are assigned from 1 to 247 (decimal). Address 0 is used as broadcast address.

If the remote TCP device is a normal Modbus server, it often is configured to respond to ANY unitId, or the specific id 0xFF (which would normally be illegal). The official Modbus TCP guide states: 

> On TCP/IP, the MODBUS server is addressed using its IP address; therefore, the MODBUS Unit Identifier is useless. The value 0xFF has to be used.
> 
> When addressing a MODBUS server connected directly to a TCP/IP network, it’s recommended not using a significant MODBUS slave address in the “Unit Identifier” field. In the event of a re-allocation of the IP addresses within an automated system and if a IP address previously assigned to a MODBUS server is then assigned to a gateway, using a significant slave address may cause trouble because of a bad routing by the gateway. Using a nonsignificant slave address, the gateway will simply discard the MODBUS PDU with no trouble. 0xFF is recommended for the “Unit Identifier" as nonsignificant value.
> 
> Remark : The value 0 is also accepted to communicate directly to a MODBUS/TCP device.

That last `Remark` implies that the `broadcast` nature of address 0 is relinquished when the device is a TCP server.

In order to best support this somewhat ambiguous system, you can register a server on the `Modbus` instance using address `0xFF`. This special address will cause that server instance to handle all requests regardless of the specified unitId UNLESS an explicit server has been set for the specific unit ID. In other words, registering a server for unit IDs 1, 2, 3 and 0xFF will have the "reasonable" consequence of units 1, 2, and 3 being handled by their respective server instances, and all other unit Ids being handled by the "wildcard" server 0xFF. When serving as a wildcard server 0xFF the server will ignore the broadcast-nature of unitID 0.

### Example TCP Client

```go
mb, err := modbus.NewTCP("server.example.com:502")
// error handling

sm := mb.GetClient(5)
// do something with client to remote unit 5
```

### Example TCP Server

When using TCP as a protocol it is normal for the Modbus Server unit to be on the "passive" side of the TCP Socket (the client will establish the TCP socket, the server side waits for a client to connect). As a consequence, it's typical for the system acting as the Modbus Server to also manage a TCP Socket service. The code is thus a little more complicated...

```go
serialId := []byte{0x01, 0x02, 0x03, 0x04, 0x05, 0x06, 0x07, 0x08}
deviceInfo := []string{"ACME Corporation", "Widget Server", "v5.6.7", "http://github.com/rolfl/modbus"}
server, err := modbus.NewServer(serialId, deviceInfo)
// Register handlers for discretes, coils, inputs, registers & files

// The `modbus.ServeAllUnits(server)` does what you expect.
tcpserv, err := modbus.NewTCPServer(":502", modbus.ServeAllUnits(server))
// Error handling

// wait for the network system to shutdown (typically wait "forever")
tcpserv.WaitClosed() 
```

With the above code, whenever a client connects to us as a service, we will establish the Mobdus protocol over the TCP socket, and then attach the supplied server as a listner for all UnitIds on the connection.

# Client operations

When behaving as a client, the client instance is able to call functions on remote servers. All the remote functions are available using names that follow the Modbus specification. All calls require a timeout parameter - calls exceeding the timeout will fail with a timeout error.

All client calls return both a data value and an error. The data value is a Go representation of the specifications response payload, as a go `struct`. All responses are `Stringers` so you can print them out to get a sensible report of the response to a client call.

Example calls as a client are:

```go
// get the server's ID - timeout after a second
serverId, err := client.ServerID(time.Second)
// error handling
fmt.Printf("The server's ID is %v", serverId)

// Write 3 coils starting from address 0 - within a second.
response, err := client.WriteMultipleCoils(0, []bool{true, false, true}, time.Second)
// error handling

```

# Server operations

When behaving as a server, the server performs a number of functions for your convenience, and additionally it abstracts out a "cache" of the memory model of the device. The memory model is a "memory safe" implementation where both your code and the modbus library code can safely read/write to the cache from different go-routines. All reads/writes are gated with an `atomic` abstraction that provides locking to the memory.

## Server-triggered commands

When the server is commanded by a client to read values (get discretes, coils, inputs, registers or files) the server will intiate an atomic read from the cache, and respond to the client. When commanded to write values (coils, registers, or files) the server will initiate an atomic transaction, and then with that atomic transaction it will request that the server handler process the mutation request. The handler call will include the current value of the memory cache, the intended write value, and it expects the return value to be the updated value to put in the cache.

Since the callback handler function also has the same atomic reference, it can query or update other parts of the memory cache as well in the same atomic action.

For example, if the following handler is registered with the server:

```go
func updateCoils(server modbus.Server, atomic modbus.Atomic, address int, values []bool, current []bool) ([]bool, error) {
	return values, nil
}

server.RegisterCoils(50, updateCoils)
```

then when the server gets a request to write values to a set of coils, it will:
1. establish an atomic lock on the server's cache
2. read the current value(s) from the specified address (as many values are intended to be written)
3. call the `updateCoils` function for the given server, with the same atomic reference, the address to write to, the values that are intended to be written to that address, and the values currently at that address (about to be overwritten)
4. the `updateCoils` function can perform whatever operations are required to change the state appropriately (including using the atomic to read/write to other places in the cache).
5. when complete, return the actual values to be stored in the cache (and if applicable, returned to the client)

See the documentation on the atomic reference for how to queue operations on the cache.

## Non-Server operations

Not all systems are triggered by client requests only. It's typical for a system to have "background" tasks that read sensors, etc. and update discretes, inputs, and even coils and holding registers. For these non-server based memory cache updates, the code still needs to perform atomic operations on the server's memory cache (in order for client reads to read the correct values).

A typical process may be for a sensor to be read periodically, and to update a register in the cache. The code would look similar to:

```go
// ... server exists

func updateServerSensorValue(val int) error {
	atomic := server.StartAtomic();
	defer atomic.Complete()
	// write a single value to holding register address 10
	return server.WriteHoldings(atomic, 10, []int{val})
}
```

Since single-operation atomic calls are common, a simpler version of the above would just be:

```go
// ... server exists

func updateServerSensorValue(val int) error {
	// write a single value to holding register address 10
	return server.WriteHoldingsAtomic(10, []int{val})
}
```
