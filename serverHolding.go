package modbus

import (
	"fmt"
)

func (s *server) x03ReadHoldingRegisters(mb Modbus, request *dataReader, response *dataBuilder) error {
	addr, _ := request.word()
	count, _ := request.word()

	atomic := s.StartAtomic()
	defer atomic.Complete()

	registers, err := s.ReadHoldings(atomic, addr, count)
	if err != nil {
		return err
	}

	// pack discretes in to bytes
	response.byte(2 * len(registers))
	response.words(registers...)
	return nil
}

func (s *server) xHoldingCommonWrite(atomic Atomic, addr int, values []int) error {
	current, err := s.ReadHoldings(atomic, addr, 1)
	if err != nil {
		return err
	}

	replacement, err := s.updateHoldings(s, atomic, addr, values, current)
	if err != nil {
		return err
	}

	// Update the cache with the replacement values
	err = s.WriteHoldings(atomic, addr, replacement)
	return err
}

func (s *server) x06WriteSingleHoldingRegister(mb Modbus, request *dataReader, response *dataBuilder) error {
	addr, _ := request.word()
	value, _ := request.word()

	atomic := s.StartAtomic()
	defer atomic.Complete()

	err := s.xHoldingCommonWrite(atomic, addr, []int{value})
	if err != nil {
		return err
	}

	response.words(addr, value)
	return nil
}

func (s *server) x10WriteHoldingRegisters(mb Modbus, request *dataReader, response *dataBuilder) error {
	addr, _ := request.word()
	count, _ := request.word()
	bcnt, err := request.byte()
	if err != nil {
		return err
	}
	if bcnt != count*2 {
		return fmt.Errorf("Expected %v bytes for %v registers, but got %v", count*2, count, bcnt)
	}
	words, err := request.words(count)
	if err != nil {
		return err
	}

	atomic := s.StartAtomic()
	defer atomic.Complete()

	err = s.xHoldingCommonWrite(atomic, addr, words)
	if err != nil {
		return err
	}

	response.words(addr, count)
	return nil
}

func (s *server) x16MaskWriteHoldingRegister(mb Modbus, request *dataReader, response *dataBuilder) error {
	addr, _ := request.word()
	andMask, _ := request.word()
	orMask, _ := request.word()

	atomic := s.StartAtomic()
	defer atomic.Complete()

	value, err := s.ReadHoldings(atomic, addr, 1)
	current := value[0]

	// The function’s algorithm is:
	// Result = (Current Contents AND And_Mask) OR (Or_Mask AND (NOT And_Mask))
	result := (current & andMask) | (orMask & ^andMask)

	err = s.xHoldingCommonWrite(atomic, addr, []int{result})
	if err != nil {
		return err
	}

	response.words(addr, andMask, orMask)
	return nil
}

func (s *server) x17WriteReadHoldingRegisters(mb Modbus, request *dataReader, response *dataBuilder) error {
	raddr, _ := request.word()
	rcount, _ := request.word()
	waddr, _ := request.word()
	wcount, _ := request.word()
	bcnt, _ := request.byte()
	if bcnt != wcount*2 {
		return fmt.Errorf("Expected %v bytes for %v registers, but got %v", wcount*2, wcount, bcnt)
	}
	words, err := request.words(wcount)
	if err != nil {
		return err
	}

	atomic := s.StartAtomic()
	defer atomic.Complete()

	err = s.xHoldingCommonWrite(atomic, waddr, words)
	if err != nil {
		return err
	}

	registers, err := s.ReadHoldings(atomic, raddr, rcount)
	if err != nil {
		return err
	}

	// pack discretes in to bytes
	response.byte(2 * len(registers))
	response.words(registers...)
	return nil
}

func (s *server) x18ReadFIFO(mb Modbus, request *dataReader, response *dataBuilder) error {
	addr, _ := request.word()

	atomic := s.StartAtomic()
	defer atomic.Complete()

	values, err := s.ReadHoldings(atomic, addr, 1)
	if err != nil {
		return err
	}
	count := values[0]
	if count > 31 {
		return IllegalValueErrorF("Fifo can have at most 31 values, not %v", count)
	}
	data, err := s.ReadHoldings(atomic, addr+1, count)
	if err != nil {
		return err
	}

	// pack discretes in to bytes
	response.words(count*2+2, count)
	response.words(data...)
	return nil
}
