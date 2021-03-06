package modbus

func (s *server) x02ReadDiscretes(mb Modbus, request *dataReader, response *dataBuilder) error {
	err := request.canRead(4)
	if err != nil {
		return err
	}
	addr, _ := request.word()
	count, _ := request.word()

	atomic := s.StartAtomic()
	defer atomic.Complete()
	discretes, err := s.ReadDiscretes(atomic, addr, count)
	if err != nil {
		return err
	}

	// pack discretes in to bytes
	response.bits(discretes...)
	return nil
}
