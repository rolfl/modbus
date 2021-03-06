package modbus

func (s *server) x01ReadCoils(mb Modbus, request *dataReader, response *dataBuilder) error {
	addr, _ := request.word()
	count, _ := request.word()

	atomic := s.StartAtomic()
	defer atomic.Complete()

	coils, err := s.ReadCoils(atomic, addr, count)
	if err != nil {
		return err
	}

	// pack discretes in to bytes
	response.bits(coils...)
	return nil
}

func (s *server) xCoilsCommonWrite(atomic Atomic, addr int, values []bool) ([]bool, error) {
	current, err := s.ReadCoils(atomic, addr, 1)
	if err != nil {
		return nil, err
	}

	replacement, err := s.updateCoils(s, atomic, addr, values, current)
	if err != nil {
		return nil, err
	}

	// Update the cache with the replacement values
	err = s.WriteCoils(atomic, addr, replacement)
	if err != nil {
		return nil, err
	}
	return replacement, nil
}

// x05WriteSingleCoil(address uint16, value bool) (PDU, error)
func (s *server) x05WriteSingleCoil(mb Modbus, request *dataReader, response *dataBuilder) error {
	addr, _ := request.word()
	value, _ := request.word()

	atomic := s.StartAtomic()
	defer atomic.Complete()

	repl, err := s.xCoilsCommonWrite(atomic, addr, []bool{value != 0})
	if err != nil {
		return err
	}

	ret := 0x0000
	if repl[0] {
		ret = 0xff00
	}

	response.word(addr)
	response.word(ret)
	return nil
}

func (s *server) x0fWriteCoils(mb Modbus, request *dataReader, response *dataBuilder) error {
	addr, _ := request.word()
	count, _ := request.word()
	coils, err := request.bits(count)
	if err != nil {
		return err
	}

	atomic := s.StartAtomic()
	defer atomic.Complete()

	repl, err := s.xCoilsCommonWrite(atomic, addr, coils)
	if err != nil {
		return err
	}

	response.word(addr)
	response.word(len(repl))
	return nil
}
