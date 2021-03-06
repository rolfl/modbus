package modbus

func (s *server) x04ReadInputRegisters(mb Modbus, request *dataReader, response *dataBuilder) error {
	err := request.canRead(4)
	if err != nil {
		return err
	}
	addr, _ := request.word()
	count, _ := request.word()

	atomic := s.StartAtomic()
	defer atomic.Complete()
	inputs, err := s.ReadInputs(atomic, addr, count)
	if err != nil {
		return err
	}

	// pack discretes in to bytes
	response.byte(2 * len(inputs))
	response.words(inputs...)
	return nil
}
