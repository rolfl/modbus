package modbus

import "fmt"

func (s *server) x07ReadExceptionStatus(mb Modbus, request *dataReader, response *dataBuilder) error {
	response.byte(0)
	return nil
}

func (s *server) x11ReportServerID(mb Modbus, request *dataReader, response *dataBuilder) error {
	tosend := bytesToInt([]byte(s.id))
	tosend = append(tosend, 0xff)
	response.nbytes(tosend...)
	return nil
}

func (s *server) x2bDeviceIdentification(mb Modbus, request *dataReader, response *dataBuilder) error {
	sfn, _ := request.byte()
	if sfn != 0x0e {
		return IllegalValueErrorF("Do not support subfunction 0x%02x. Only Device Identification 0x0e", sfn)
	}
	err := request.canRead(2)
	if err != nil {
		return err
	}
	code, _ := request.byte()
	oid, _ := request.byte()
	if code < 1 || code > 4 {
		return IllegalValueErrorF("No such code %v for Device Identification", code)
	}

	if oid >= 0x07 && oid < 0x80 {
		return IllegalValueErrorF("Illegal ObjectId %v for Device Identification", oid)
	}

	origid := oid
	if oid >= 0x80 {
		oid = oid - 0x80 + 7
	}

	if oid >= len(s.deviceInfo) {
		return IllegalValueErrorF("No such ObjectId %v for Device Identification", origid)
	}

	if (code == 1 && oid > 2) || (code == 2 && (oid <= 2 || oid > 7)) || (code == 3 && oid <= 7) {
		return IllegalValueErrorF("Cannot get object ID %v with code %v", origid, code)
	}

	limits := []int{0, 3, 7, oid + 1, oid + 1}
	max := limits[code]
	if max > len(s.deviceInfo) {
		max = len(s.deviceInfo)
	}

	conf := 1
	if len(s.deviceInfo) > 3 {
		conf = 2
	}
	if len(s.deviceInfo) > 7 {
		conf = 3
	}
	conf += 0x80

	tosend := s.deviceInfo[oid:max]
	remaining := 252
	sent := make([][]byte, 0, len(tosend))
	for _, di := range tosend {
		dib := []byte(di)
		diz := len(dib) + 1
		if remaining < diz {
			break
		}
		remaining -= diz
		sent = append(sent, dib)
	}

	more := 0
	next := 0
	if len(tosend) > len(sent) {
		more = 0xff
		next = origid + len(sent)
	}
	response.byte(0x0e)
	response.byte(code)
	response.byte(conf)
	response.byte(more)
	response.byte(next)
	response.byte(len(sent))
	for i, b := range sent {
		response.byte(i + origid)
		response.nbytes(bytesToInt(b)...)
	}
	return nil
}

func (s *server) x08Diagnostic(mb Modbus, request *dataReader, response *dataBuilder) error {
	subfn, _ := request.word()
	response.word(subfn)
	switch subfn {
	case 0x00:
		return s.diagEcho(request, response)
	case 0x01:
		return s.diagRestartComm(request, response)
	case 0x02:
		return s.diagRegister(request, response)
	case 0x0a:
		return s.diagClearCounters(mb, request, response)
	case 0x0b:
		return s.diagGenericCount("Bus Messages", mb.Diagnostics().Messages, request, response)
	case 0x0c:
		return s.diagGenericCount("Bus Errors", mb.Diagnostics().CommErrors, request, response)
	case 0x0d:
		return s.diagGenericCount("Bus Exceptions", mb.Diagnostics().Exceptions, request, response)
	case 0x0e:
		return s.diagGenericCount("Server Messages", s.diag.getDiagnostics().Messages, request, response)
	case 0x0f:
		return s.diagGenericCount("Server No Response", s.diag.getDiagnostics().NoResponse, request, response)
	case 0x10:
		return s.diagGenericCount("Server NAK", s.diag.getDiagnostics().ServerNAKs, request, response)
	case 0x11:
		return s.diagGenericCount("Server Busy", s.diag.getDiagnostics().ServerBusy, request, response)
	case 0x12:
		return s.diagGenericCount("Bus Overruns", mb.Diagnostics().Overruns, request, response)
	case 0x14:
		return s.diagClearOverrunCounter(mb, request, response)
	}
	return IllegalValueErrorF("Unsupported diagnostic sub function %v", subfn)
}

func (s *server) diagEcho(request *dataReader, response *dataBuilder) error {
	words, err := request.words((len(request.data) / 2) - 1)
	if err != nil {
		return err
	}
	response.words(words...)
	return nil
}

func (s *server) diagRestartComm(request *dataReader, response *dataBuilder) error {
	code, err := request.word()
	if err != nil {
		return err
	}
	// TODO Restart comm - not applicable for this server, just ignore it....
	response.word(code)
	return nil
}

func (s *server) diagRegister(request *dataReader, response *dataBuilder) error {
	check, err := request.word()
	if err != nil {
		return err
	}
	if check != 0 {
		return fmt.Errorf("diagRegister requires 0x0000 input")
	}
	// TODO Restart comm - not applicable for this server, just ignore it....
	response.word(0)
	return nil
}

func (s *server) diagClearCounters(mb Modbus, request *dataReader, response *dataBuilder) error {
	check, err := request.word()
	if err != nil {
		return err
	}
	if check != 0 {
		return fmt.Errorf("diagClearCounters requires 0x0000 input")
	}
	s.diag.clear()
	mb.clearDiagnostics()
	response.word(0)
	return nil
}

func (s *server) diagClearOverrunCounter(mb Modbus, request *dataReader, response *dataBuilder) error {
	check, err := request.word()
	if err != nil {
		return err
	}
	if check != 0 {
		return fmt.Errorf("diagClearOverrunCounter requires 0x0000 input")
	}
	s.diag.clear()
	mb.clearOverrunCounter()
	response.word(0)
	return nil
}

func (s *server) diagGenericCount(name string, val int, request *dataReader, response *dataBuilder) error {
	check, err := request.word()
	if err != nil {
		return err
	}
	if check != 0 {
		return fmt.Errorf("%v requires 0x0000 input", name)
	}
	cnt := wordClamp(val)
	response.word(cnt)
	return nil
}

func (s *server) x0bCommEventCounter(mb Modbus, request *dataReader, response *dataBuilder) error {
	busy := 0x0000
	if s.Busy() {
		busy = 0xffff
	}

	response.word(busy)
	response.word(wordClamp(s.diag.getDiagnostics().EventCounter))
	return nil
}

func (s *server) x0cCommEventLog(mb Modbus, request *dataReader, response *dataBuilder) error {
	diag := s.diag.getDiagnostics()
	busy := 0x0000
	if s.Busy() {
		busy = 0xffff
	}
	events := mb.getEventLog()
	response.byte(len(events) + 6)
	response.word(busy)
	response.word(wordClamp(diag.EventCounter))
	response.word(wordClamp(diag.Messages))
	response.bytes(events...)
	return nil
}
