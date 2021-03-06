package modbus

type fileReadRequest struct {
	file    int
	address int
	count   int
}

type fileWriteRequest struct {
	file    int
	address int
	values  []int
}

func (s *server) x14ReadFileRecord(mb Modbus, request *dataReader, response *dataBuilder) error {
	size, err := request.byte()
	err = request.canRead(size)
	if err != nil {
		return err
	}
	if size%7 != 0 {
		return IllegalValueErrorF("Expected subrequest size to be a multiple of 7, not %v", size)
	}

	xsize := 1
	reqs := make([]fileReadRequest, size/7)
	for i := range reqs {
		code, _ := request.byte()
		if code != 0x06 {
			return IllegalValueErrorF("Expected SubRequest reference 0x06 but got 0x%02x", code)
		}
		file, _ := request.word()
		addr, _ := request.word()
		count, _ := request.word()
		reqs[i] = fileReadRequest{file, addr, count}
		xsize += 2 + count*2
	}

	if xsize > 253 { //(PDU limit)
		return IllegalFunctionErrorF("File Record Requests will exceed limit of payload, max 253, requested %v", xsize)
	}

	atomic := s.StartAtomic()
	defer atomic.Complete()

	response.byte(xsize)
	for _, req := range reqs {
		recs, err := s.ReadFileRecords(atomic, req.file, req.address, req.count)
		if err != nil {
			return err
		}
		response.byte(1 + len(recs)*2)
		response.byte(0x06)
		response.words(recs...)
	}

	return nil
}

func (s *server) x15WriteFileRecord(mb Modbus, request *dataReader, response *dataBuilder) error {
	size, err := request.byte()
	err = request.canRead(size)
	if err != nil {
		return err
	}

	reqs := make([]fileWriteRequest, 0)
	for request.cursor < size {
		err = request.canRead(7)
		if err != nil {
			return err
		}
		code, _ := request.byte()
		if code != 0x06 {
			return IllegalValueErrorF("Expected SubRequest reference 0x06 but got 0x%02x", code)
		}
		file, _ := request.word()
		addr, _ := request.word()
		count, _ := request.word()

		values, err := request.words(count)
		if err != nil {
			return err
		}
		reqs = append(reqs, fileWriteRequest{file, addr, values})
	}

	atomic := s.StartAtomic()
	defer atomic.Complete()

	response.byte(size)
	for _, req := range reqs {
		current, err := s.ReadFileRecords(atomic, req.file, req.address, len(req.values))
		if err != nil {
			return err
		}

		repl, err := s.updateFiles(s, atomic, req.file, req.address, req.values, current)
		err = s.WriteFileRecords(atomic, req.file, req.address, repl)
		if err != nil {
			return err
		}
		response.byte(0x06)
		response.words(req.file, req.address, len(req.values))
		response.words(repl...)
	}

	return nil
}
