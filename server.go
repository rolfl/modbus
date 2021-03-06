package modbus

import (
	"fmt"
)

/*
Atomic allows locked access to the server's internal cache of coil, discrete, input, holding, and file values.
implementation in serverCache.go An Atomic instance is created by calling the StartAtomic() function on the Server

Do not Complete an atomic unless you started it. It's normal to `defer a.Complete()` immediately after starting it

	atomic := server.StartAtomic()
	defer atomic.Complete()

	// do stuff using the atomic...

*/
type Atomic interface {
	// Complete indicates that all operations in the atomic set are queued. It returns when all operations have completed.
	Complete()

	execute(func())
}

// UpdateCoils is a function called when coils are expected to be written by request from a remote client
// Do not Complete the atomic
type UpdateCoils func(server Server, atomic Atomic, address int, values []bool, current []bool) ([]bool, error)

// UpdateHoldings is a function called when holding registers are expected to be written by request from a remote client
// Do not Complete the atomic
type UpdateHoldings func(server Server, atomic Atomic, address int, values []int, current []int) ([]int, error)

// UpdateFile is a function called when files are expected to be written by request from a remote client
// Do not Complete the atomic
type UpdateFile func(server Server, atomic Atomic, file int, address int, values []int, current []int) ([]int, error)

// Server represents a system that can handle an incoming request from a remote client
type Server interface {
	// Diagnostics returns the current diagnostic counts of the server instance
	Diagnostics() ServerDiagnostics

	// Busy will return true if a command is actively being handled
	Busy() bool

	// StartAtomic requests that access to the internal memory model/cache (coils, registers, discretes, inputs and files)
	// of the Server is granted. Only 1 transaction is active at a time, and is active until it is Completed.
	StartAtomic() Atomic

	// RegisterDiscretes indicates how many discretes to make available in the server memory model/cache
	RegisterDiscretes(count int)
	// ReadDiscretes performs a discrete read operation as part of an existing atomic operation from the memory model/cache
	ReadDiscretes(atomic Atomic, address int, count int) ([]bool, error)
	// ReadDiscretesAtomic performs an atomic ReadDiscretes
	ReadDiscretesAtomic(address int, count int) ([]bool, error)
	// WriteDiscretes performs a discrete write operation as part of an existing atomic operation to the memory model/cache
	WriteDiscretes(atomic Atomic, address int, values []bool) error
	// WriteDiscretesAtomic performs an atomic WriteDiscretes
	WriteDiscretesAtomic(address int, values []bool) error

	// RegisterCoils indicates how many coils to make available in the server memory model/cache, and which function to call
	// when a remote client attempts to update the coil settings
	RegisterCoils(count int, handler UpdateCoils)
	// ReadCoils performs a coil read operation as part of an existing atomic operation from the memory model/cache
	ReadCoils(atomic Atomic, address int, count int) ([]bool, error)
	// ReadCoilsAtomic performs an atomic ReadCoils
	ReadCoilsAtomic(address int, count int) ([]bool, error)
	// WriteCoils performs a coil write operation as part of an existing atomic operation to the memory model/cache
	WriteCoils(atomic Atomic, address int, values []bool) error
	// WriteCoilsAtomic performs an atomic WriteCoils
	WriteCoilsAtomic(address int, values []bool) error

	// RegisterInputs indicates how many inputs to make available in the server memory model/cache
	RegisterInputs(count int)
	// ReadInputs performs ain input read operation as part of an existing atomic operation from the memory model/cache
	ReadInputs(atomic Atomic, address int, count int) ([]int, error)
	// ReadInputsAtomic performs an atomic ReadInputs
	ReadInputsAtomic(address int, count int) ([]int, error)
	// WriteInputs performs an input write operation as part of an existing atomic operation to the memory model/cache
	WriteInputs(atomic Atomic, address int, values []int) error
	// WriteInputsAtomic performs an atomic WriteInputs
	WriteInputsAtomic(address int, values []int) error

	// RegisterHoldings indicates how many coils to make available in the server memory model/cache, and which function to call
	// when a remote client attempts to update the holding register values
	RegisterHoldings(count int, handler UpdateHoldings)
	// ReadHoldings performs a holding register read operation as part of an existing atomic operation from the memory model/cache
	ReadHoldings(atomic Atomic, address int, count int) ([]int, error)
	// ReadHoldingsAtomic performs an atomic ReadHoldings
	ReadHoldingsAtomic(address int, count int) ([]int, error)
	// WriteHoldings performs a holding register write operation as part of an existing atomic operation to the memory model/cache
	WriteHoldings(atomic Atomic, address int, values []int) error
	// WriteHoldingsAtomic performs an atomic WriteHoldings
	WriteHoldingsAtomic(address int, values []int) error

	// RegisterFiles indicates how many files to make available in the server memory model/cache, and which function to call
	// when a remote client attempts to update the file records
	RegisterFiles(count int, handler UpdateFile)
	// ReadFileRecords performs a file records read operation as part of an existing atomic operation from the memory model/cache
	ReadFileRecords(atomic Atomic, address int, offset int, count int) ([]int, error)
	// ReadFileRecordsAtomic performs an atomic ReadFileRecords
	ReadFileRecordsAtomic(address int, offset int, count int) ([]int, error)
	// WriteFileRecords performs a file records write operation as part of an existing atomic operation to the memory model/cache
	WriteFileRecords(atomic Atomic, address int, offset int, values []int) error
	// WriteFileRecordsAtomic performs an atomic WriteFileRecords
	WriteFileRecordsAtomic(address int, offset int, values []int) error

	// request is called from the modbus layer and instructs the server to handle a request.
	request(bus Modbus, unit byte, function byte, data []byte) ([]byte, error)
}

type requestHandler func(Modbus, *dataReader, *dataBuilder) error

type checkHandler func() error

type requestHandlerMeta struct {
	function byte
	minSize  int
	handler  requestHandler
	event    bool
}

func (rhm requestHandlerMeta) notEvent() {
	rhm.event = false
}

type server struct {
	id             []byte
	deviceInfo     []string
	rhandlers      map[byte]requestHandlerMeta
	discretes      []bool
	coils          []bool
	inputs         []int
	holdings       []int
	files          [][]int
	atomics        chan Atomic
	diag           *serverDiagnosticManager
	updateCoils    UpdateCoils
	updateHoldings UpdateHoldings
	updateFiles    UpdateFile
}

// NewServer creates a Server instance that can be bound to a Modbus instance using modbus.SetServer(...).
func NewServer(id []byte, deviceInfo []string) (Server, error) {
	if len(deviceInfo) < 3 {
		return nil, fmt.Errorf("DeviceInfo is required to have at least 3 members, not %v", deviceInfo)
	}
	s := &server{}
	s.id = make([]byte, len(id))
	copy(s.id, id)
	s.deviceInfo = make([]string, len(deviceInfo))
	copy(s.deviceInfo, deviceInfo)
	s.rhandlers = make(map[byte]requestHandlerMeta)
	s.diag = newServerDiagnosticManager()
	s.atomics = make(chan Atomic, 0)

	// Set up the discrete handlers
	s.addRequestHandler(0x02, 4, s.x02ReadDiscretes)

	// Set up the coil handlers
	s.addRequestHandler(0x01, 4, s.x01ReadCoils)
	s.addRequestHandler(0x05, 4, s.x05WriteSingleCoil)
	s.addRequestHandler(0x0f, 4, s.x0fWriteCoils)

	// Set up the input handlers
	s.addRequestHandler(0x04, 4, s.x04ReadInputRegisters)

	// Set up the holding register handlers
	s.addRequestHandler(0x03, 4, s.x03ReadHoldingRegisters)
	s.addRequestHandler(0x06, 4, s.x06WriteSingleHoldingRegister)
	s.addRequestHandler(0x10, 4, s.x10WriteHoldingRegisters)
	s.addRequestHandler(0x16, 6, s.x16MaskWriteHoldingRegister)
	s.addRequestHandler(0x17, 9, s.x17WriteReadHoldingRegisters)
	s.addRequestHandler(0x18, 2, s.x18ReadFIFO)

	// Set up the diagnostic handlers
	s.addRequestHandler(0x07, 0, s.x07ReadExceptionStatus).notEvent()
	s.addRequestHandler(0x2b, 1, s.x2bDeviceIdentification).notEvent()
	s.addRequestHandler(0x11, 0, s.x11ReportServerID).notEvent()
	s.addRequestHandler(0x08, 2, s.x08Diagnostic).notEvent()
	s.addRequestHandler(0x0b, 0, s.x0bCommEventCounter).notEvent()
	s.addRequestHandler(0x0c, 0, s.x0cCommEventLog).notEvent()

	// Set up the File handlers
	s.addRequestHandler(0x14, 1, s.x14ReadFileRecord).notEvent()
	s.addRequestHandler(0x15, 8, s.x15WriteFileRecord).notEvent()

	go s.manageCache()

	return s, nil
}

func (s *server) addRequestHandler(function byte, minsize int, handler requestHandler) requestHandlerMeta {
	ret := requestHandlerMeta{function, minsize, handler, true}
	s.rhandlers[function] = ret
	return ret
}

func (s *server) Diagnostics() ServerDiagnostics {
	return s.diag.getDiagnostics()
}

func (s *server) Busy() bool {
	return s.diag.busy()
}

func (s *server) RegisterDiscretes(count int) {
	atomic := s.StartAtomic()
	defer atomic.Complete()
	s.ensureDiscretes(atomic, count)
}

func (s *server) RegisterCoils(count int, handler UpdateCoils) {
	atomic := s.StartAtomic()
	defer atomic.Complete()
	s.ensureCoils(atomic, count)
	s.updateCoils = handler
}

func (s *server) RegisterInputs(count int) {
	atomic := s.StartAtomic()
	defer atomic.Complete()
	s.ensureInputs(atomic, count)
}

func (s *server) RegisterHoldings(count int, handler UpdateHoldings) {
	atomic := s.StartAtomic()
	defer atomic.Complete()
	s.ensureHoldings(atomic, count)
	s.updateHoldings = handler
}

func (s *server) RegisterFiles(count int, handler UpdateFile) {
	atomic := s.StartAtomic()
	defer atomic.Complete()
	s.ensureFiles(atomic, count)
	s.updateFiles = handler
}

func (s *server) request(mb Modbus, unit byte, function byte, request []byte) ([]byte, error) {
	h, ok := s.rhandlers[function]
	if !ok {
		return nil, fmt.Errorf("Function code 0x%02x not implemented", function)
	}

	s.diag.message()
	if h.event {
		s.diag.eventQueued()
		defer s.diag.eventComplete()
	}

	req := getReader(request)
	res := dataBuilder{}

	err := req.canRead(h.minSize)
	if err != nil {
		return nil, err
	}

	err = h.handler(mb, &req, &res)
	if err != nil {
		return nil, err
	}

	err = req.remaining()
	if err != nil {
		return nil, err
	}

	if h.event {
		// a successful recorded event increments the successful event counter
		s.diag.eventCounter()
	}

	return res.payload(), nil
}
