# modbus
Modbus driver

This code depends on tarm/serial to drive the serial port, but that library does not have DTR support.
The plan is to contribute back the DTR (and possibly RTS) support back to tarm/serial, but it needs to work
on Linux first.

Decent libraries for serial are hard to come by. tarm/serial is the best for this purpose because:

- no cgo dependency (mikepb/go-serial) was considered, but compiling it is a real challenge
- it is well established (lots of eyes on it)
- it is relatively thin ..... not much added features
- it supports read timeouts

## Design

This code has been put together separating out the various layers of Modbus, with RTU specific code in one
place and more general code above it. Currently only RTU is supported, but anticipate TCP to come.

TCP supports a "bridge" device at the end of the TCP socket, so in practice, each TCP-server is really collection
of up to 247 actual devices, and the TCP modbus protocol allows regular unit addressing (1-247) in the TCP payload.
What this means, is that each TCP endpoint can be treated like a full RTU bus (the bus has a TCP address, and each
device behind that TCP address has a unit address)

This code has a concept of a WireProtocol - the first one being RTU with TCP being planned. For each COM/serial device
you can create a ModbusRTU instance on that WireProtocol with code like:

```go
modbus, err := modbus.NewModbusRTU("COM#", 115200, 'N', 1, true)
...
```

Which establishes a Modbus instance over RTU with the given port properties and DTR set

Expect something simiar for TCP ... which will create an "identical" modbus instance except talking via the TCP protocol.
In effect, and IPAddress/Port combination for TCP is the same as the RTU `COMX` or `/dev/tty?` ... each one exposes a bus
of modbus units

The code allows you to create as many instances of the Modbus driver as you have remote busses (TCP addresses or Serial
devices), within the limits of your memory.

Once you have a driver instance, you can get a Client instance to a specific unit on that bus. With a client, you can
then direct regular Modbus functions to that specific client.

Example code to show Modbus features is:

```go

	fmt.Printf("Starting Modbus driver\n")
	mb, err := modbus.NewModbusRTU("COM5", 115200, 'N', 1, true)
	if err != nil {
		fmt.Printf("Error opening modbus: %v\n", err)
		return
	}
	defer mb.Close()

    // get a client to unit 5
	client, err := mb.GetClient(5)
	if err != nil {
		fmt.Printf("Unable to retrieve Client: %v\n", err)
		return
	}

    // start a request to get the server's serial/server ID
    serveridRequest := client.ServerID(time.Second * 10)
    err = <-serveridRequest.ERR
    if err != nil {
        fmt.Printf("Unable to get server's ID: %v\n", err)
        return
    }

    fmt.Println(serveridRequest)
```