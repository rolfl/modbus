package modbus

type atomic struct {
	todo chan func()
	done chan bool
}

func (a *atomic) execute(fn func()) {
	a.todo <- fn
}

func (a *atomic) Complete() {
	close(a.todo)
	<-a.done
}

func (s *server) StartAtomic() Atomic {
	atomic := <-s.atomics
	return atomic
}

// manageCache is run as a go-routine, it's the only one that accesses the discretes/coils/inputs/registers/files cache
func (s *server) manageCache() {
	for {
		// seed the channel with a new atomic operation.
		// the chan supports a buffer of 5 functions to run... we don't expect to ever have more than 1, but whatever
		a := &atomic{make(chan func(), 5), make(chan bool)}
		s.atomics <- a

		// while there are atomic operations, handle them.
		for fn := range a.todo {
			fn()
		}
		close(a.done)
		// the channel was closed, no more atomics, get ready to set up another seed.
	}
}

func (s *server) ensureDiscretes(atomic Atomic, count int) {
	done := make(chan bool)
	atomic.execute(func() {
		defer close(done)
		if len(s.discretes) < count {
			s.discretes = append(s.discretes, make([]bool, count-len(s.discretes))...)
		}
	})
	<-done
}

func (s *server) ensureCoils(atomic Atomic, count int) {
	done := make(chan bool)
	atomic.execute(func() {
		defer close(done)
		if len(s.coils) < count {
			s.coils = append(s.coils, make([]bool, count-len(s.coils))...)
		}
	})
	<-done
}

func (s *server) ensureInputs(atomic Atomic, count int) {
	done := make(chan bool)
	atomic.execute(func() {
		defer close(done)
		if len(s.inputs) < count {
			s.inputs = append(s.inputs, make([]int, count-len(s.inputs))...)
		}
	})
	<-done
}

func (s *server) ensureHoldings(atomic Atomic, count int) {
	done := make(chan bool)
	atomic.execute(func() {
		defer close(done)
		if len(s.holdings) < count {
			s.holdings = append(s.holdings, make([]int, count-len(s.holdings))...)
		}
	})
	<-done
}

func (s *server) ensureFiles(atomic Atomic, count int) {
	done := make(chan bool)
	atomic.execute(func() {
		defer close(done)
		if len(s.files) < count {
			s.files = append(s.files, make([][]int, count-len(s.files))...)
		}
	})
	<-done
}

func (s *server) ReadDiscretes(atomic Atomic, address, count int) ([]bool, error) {
	cret := make(chan []bool)
	cerr := make(chan error)
	atomic.execute(func() {
		defer close(cret)
		defer close(cerr)
		err := serverCheckAddress("Discrete", address, count, len(s.discretes))
		if err != nil {
			cerr <- err
		} else {
			cret <- append(make([]bool, 0), s.discretes[address:address+count]...)
		}
	})
	if ret, ok := <-cret; ok {
		return ret, nil
	}
	err := <-cerr
	return nil, err
}

func (s *server) ReadDiscretesAtomic(address int, count int) ([]bool, error) {
	atomic := s.StartAtomic()
	defer atomic.Complete()
	return s.ReadDiscretes(atomic, address, count)
}

func (s *server) ReadCoils(atomic Atomic, address, count int) ([]bool, error) {
	cret := make(chan []bool)
	cerr := make(chan error)
	atomic.execute(func() {
		defer close(cret)
		defer close(cerr)
		err := serverCheckAddress("Coil", address, count, len(s.coils))
		if err != nil {
			cerr <- err
		} else {
			cret <- append(make([]bool, 0), s.coils[address:address+count]...)
		}
	})
	if ret, ok := <-cret; ok {
		return ret, nil
	}
	err := <-cerr
	return nil, err
}

func (s *server) ReadCoilsAtomic(address int, count int) ([]bool, error) {
	atomic := s.StartAtomic()
	defer atomic.Complete()
	return s.ReadCoils(atomic, address, count)
}

func (s *server) ReadInputs(atomic Atomic, address, count int) ([]int, error) {
	cret := make(chan []int)
	cerr := make(chan error)
	atomic.execute(func() {
		defer close(cret)
		defer close(cerr)
		err := serverCheckAddress("Input", address, count, len(s.inputs))
		if err != nil {
			cerr <- err
		} else {
			cret <- append(make([]int, 0), s.inputs[address:address+count]...)
		}
	})
	if ret, ok := <-cret; ok {
		return ret, nil
	}
	err := <-cerr
	return nil, err
}

func (s *server) ReadInputsAtomic(address int, count int) ([]int, error) {
	atomic := s.StartAtomic()
	defer atomic.Complete()
	return s.ReadInputs(atomic, address, count)
}

func (s *server) ReadHoldings(atomic Atomic, address, count int) ([]int, error) {
	cret := make(chan []int)
	cerr := make(chan error)
	atomic.execute(func() {
		defer close(cret)
		defer close(cerr)
		err := serverCheckAddress("Holding", address, count, len(s.holdings))
		if err != nil {
			cerr <- err
		} else {
			cret <- append(make([]int, 0), s.holdings[address:address+count]...)
		}
	})
	if ret, ok := <-cret; ok {
		return ret, nil
	}
	err := <-cerr
	return nil, err
}

func (s *server) ReadHoldingsAtomic(address int, count int) ([]int, error) {
	atomic := s.StartAtomic()
	defer atomic.Complete()
	return s.ReadHoldings(atomic, address, count)
}

func (s *server) ReadFileRecords(atomic Atomic, file int, address int, count int) ([]int, error) {
	cret := make(chan struct {
		values []int
		err    error
	})
	atomic.execute(func() {
		defer close(cret)
		err := serverCheckAddress("File", file, 1, len(s.files))
		toSend := make([]int, 0)
		if err == nil {
			f := s.files[file]
			if len(f) > address {
				available := len(f) - address
				if available < count {
					count = available
				}
				toSend = make([]int, count)
				copy(toSend, f[address:address+count])
			}
		}
		cret <- struct {
			values []int
			err    error
		}{toSend, err}
	})
	got := <-cret
	if got.err != nil {
		return nil, got.err
	}
	return got.values, nil
}

func (s *server) ReadFileRecordsAtomic(file int, address, count int) ([]int, error) {
	atomic := s.StartAtomic()
	defer atomic.Complete()
	return s.ReadFileRecords(atomic, file, address, count)
}

func (s *server) WriteDiscretes(atomic Atomic, address int, values []bool) error {
	count := len(values)
	cerr := make(chan error)
	atomic.execute(func() {
		defer close(cerr)
		err := serverCheckAddress("Discrete", address, count, len(s.discretes))
		if err != nil {
			cerr <- err
		} else {
			copy(s.discretes[address:address+count], values)
		}
	})
	err := <-cerr
	return err
}

func (s *server) WriteDiscretesAtomic(address int, values []bool) error {
	atomic := s.StartAtomic()
	defer atomic.Complete()
	return s.WriteDiscretes(atomic, address, values)
}

func (s *server) WriteCoils(atomic Atomic, address int, values []bool) error {
	count := len(values)
	cerr := make(chan error)
	atomic.execute(func() {
		defer close(cerr)
		err := serverCheckAddress("Coil", address, count, len(s.coils))
		if err != nil {
			cerr <- err
		} else {
			copy(s.coils[address:address+count], values)
		}
	})
	err := <-cerr
	return err
}

func (s *server) WriteCoilsAtomic(address int, values []bool) error {
	atomic := s.StartAtomic()
	defer atomic.Complete()
	return s.WriteCoils(atomic, address, values)
}

func (s *server) WriteInputs(atomic Atomic, address int, values []int) error {
	count := len(values)
	cerr := make(chan error)
	atomic.execute(func() {
		defer close(cerr)
		err := serverCheckAddress("Input", address, count, len(s.inputs))
		if err != nil {
			cerr <- err
		} else {
			copy(s.inputs[address:address+count], values)
		}
	})
	err := <-cerr
	return err
}

func (s *server) WriteInputsAtomic(address int, values []int) error {
	atomic := s.StartAtomic()
	defer atomic.Complete()
	return s.WriteInputs(atomic, address, values)
}

func (s *server) WriteHoldings(atomic Atomic, address int, values []int) error {
	count := len(values)
	cerr := make(chan error)
	atomic.execute(func() {
		defer close(cerr)
		err := serverCheckAddress("Holding", address, count, len(s.holdings))
		if err != nil {
			cerr <- err
		} else {
			copy(s.holdings[address:address+count], values)
		}
	})
	err := <-cerr
	return err
}

func (s *server) WriteHoldingsAtomic(address int, values []int) error {
	atomic := s.StartAtomic()
	defer atomic.Complete()
	return s.WriteHoldings(atomic, address, values)
}

func (s *server) WriteFileRecords(atomic Atomic, file int, address int, values []int) error {
	count := len(values)
	cerr := make(chan error)
	atomic.execute(func() {
		defer close(cerr)
		err := serverCheckAddress("File", file, 1, len(s.files))
		if err != nil {
			cerr <- err
			return
		}
		err = serverCheckAddress("FileRecord", address, len(values), 10000)
		if err != nil {
			cerr <- err
			return
		}
		f := s.files[file]

		currentLen := len(f)
		pre := f[:currentLen]
		pad := make([]int, 0)
		if currentLen < address {
			pad = make([]int, address-currentLen)
		} else {
			pre = s.files[file][:address]
		}
		vlen := address + count
		nlen := vlen
		post := make([]int, 0)
		if nlen < currentLen {
			nlen = currentLen
			post = f[vlen:]
		}

		nfile := make([]int, nlen)
		copy(nfile, pre)
		copy(nfile[len(pre):], pad)
		copy(nfile[address:], values)
		copy(nfile[vlen:], post)
		s.files[file] = nfile
	})
	err := <-cerr
	return err
}

func (s *server) WriteFileRecordsAtomic(address int, offset int, values []int) error {
	atomic := s.StartAtomic()
	defer atomic.Complete()
	return s.WriteFileRecords(atomic, address, offset, values)
}
