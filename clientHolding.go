package modbus

import (
	"fmt"
	"strings"
	"time"
)

// X03xReadHolding server response to a Read Multiple Holding Registers request
type X03xReadHolding struct {
	Address int
	Values  []int
}

func (s X03xReadHolding) String() string {
	cnt := len(s.Values)
	txt := make([]string, cnt)
	for i, v := range s.Values {
		txt[i] = fmt.Sprintf("    0x%04x:   0x%04x  % 6d\n", s.Address+i, v, v)
	}
	return fmt.Sprintf("X03xReadHolding %05d -> %05d (count %v)\n", s.Address, s.Address+cnt-1, cnt) + strings.Join(txt, "")
}

func (c client) ReadHoldings(from int, count int, tout time.Duration) (*X03xReadHolding, error) {
	p := dataBuilder{}
	p.word(from)
	p.word(count)
	ret := &X03xReadHolding{}
	tx := pdu{0x03, p.payload()}
	decode := func(r *dataReader) error {
		l, err := r.byte()
		if err != nil {
			return err
		}
		if l != count*2 {
			return fmt.Errorf("Expect Read Holding Registers response to have correct count of values, %v not %v", count, l/2)
		}
		v, err := r.words(count)
		if err != nil {
			return err
		}
		ret.Address = from
		ret.Values = v
		return nil
	}
	err := <-c.query(tout, tx, decode)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// X06xWriteSingleHolding server response to a Read Multiple Holding Registers request
type X06xWriteSingleHolding struct {
	Address int
	Value   int
}

func (s X06xWriteSingleHolding) String() string {
	return fmt.Sprintf("X06xWriteSingleHolding 0x%04x:   0x%04x  % 6d", s.Address, s.Value, s.Value)
}

func (c client) WriteSingleHolding(address int, value int, tout time.Duration) (*X06xWriteSingleHolding, error) {
	p := dataBuilder{}
	p.word(address)
	p.word(value)
	ret := &X06xWriteSingleHolding{}
	tx := pdu{0x06, p.payload()}
	decode := func(r *dataReader) error {
		got, err := r.word()
		if err != nil {
			return err
		}
		if got != address {
			return fmt.Errorf("Expect Write Single Holding Registers response to for the same address %v, not %v", address, got)
		}
		val, err := r.word()
		if err != nil {
			return err
		}
		if val != value {
			return fmt.Errorf("Expect Write Single Holding Registers response to for the same value %v, not %v", value, val)
		}
		ret.Address = address
		ret.Value = val
		return nil
	}
	err := <-c.query(tout, tx, decode)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// X10xWriteMultipleHoldings server response to a Write Multiple Holding Registers request
type X10xWriteMultipleHoldings struct {
	Address int
	Count   int
}

func (s X10xWriteMultipleHoldings) String() string {
	return fmt.Sprintf("X10xWriteMultipleHoldings 0x%04x: count %d", s.Address, s.Count)
}

func (c client) WriteMultipleHoldings(address int, values []int, tout time.Duration) (*X10xWriteMultipleHoldings, error) {
	p := dataBuilder{}
	p.word(address)
	p.word(len(values))
	p.byte(len(values) * 2)
	p.words(values...)
	tx := pdu{0x10, p.payload()}
	ret := &X10xWriteMultipleHoldings{}
	decode := func(r *dataReader) error {
		got, err := r.word()
		if err != nil {
			return err
		}
		if got != address {
			return fmt.Errorf("Expect Write Multiple Holding Registers response to for the same address %v, not %v", address, got)
		}
		set, err := r.word()
		if err != nil {
			return err
		}
		if set != len(values) {
			return fmt.Errorf("Expect Write Multiple Holding Registers response to for the same value count %v, not %v", len(values), set)
		}
		ret.Address = address
		ret.Count = set
		return nil
	}
	err := <-c.query(tout, tx, decode)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// X17xWriteReadHoldings server response to a Write/Read Multiple Holding Registers request
type X17xWriteReadHoldings struct {
	Address int
	Values  []int
}

func (s X17xWriteReadHoldings) String() string {
	cnt := len(s.Values)
	txt := make([]string, cnt)
	for i, v := range s.Values {
		txt[i] = fmt.Sprintf("    0x%04x:   0x%04x  % 6d\n", s.Address+i, v, v)
	}
	return fmt.Sprintf("X17xReadWriteHoldings %05d -> %05d (count %v)\n", s.Address, s.Address+cnt-1, cnt) + strings.Join(txt, "")
}

func (c client) WriteReadMultipleHoldings(read int, count int, write int, values []int, tout time.Duration) (*X17xWriteReadHoldings, error) {
	p := dataBuilder{}
	p.word(read)
	p.word(count)
	p.word(write)
	p.word(len(values))
	p.byte(len(values) * 2)
	p.words(values...)
	tx := pdu{0x17, p.payload()}
	ret := &X17xWriteReadHoldings{}
	decode := func(r *dataReader) error {
		l, err := r.byte()
		if err != nil {
			return err
		}
		if len(r.data) != l+1 {
			return fmt.Errorf("Expect Read/Write Holding Registers response to have correct internal length, %v not %v", len(r.data)-1, l)
		}
		if l != count*2 {
			return fmt.Errorf("Expect Read/Write Holding Registers response to have correct count of values, %v not %v", count, l/2)
		}
		v, err := r.words(count)
		if err != nil {
			return err
		}
		ret.Address = read
		ret.Values = v
		return nil
	}
	err := <-c.query(tout, tx, decode)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// X16xMaskWriteHolding server response to a Read Multiple Holding Registers request
type X16xMaskWriteHolding struct {
	Address int
	ANDMask int
	ORMask  int
}

func (s X16xMaskWriteHolding) String() string {
	return fmt.Sprintf("X16xMaskWriteHolding 0x%04x:  AND 0x%04x  OR  0x%04x", s.Address, s.ANDMask, s.ORMask)
}

func (c client) MaskWriteHolding(address int, andmask int, ormask int, tout time.Duration) (*X16xMaskWriteHolding, error) {
	p := dataBuilder{}
	p.word(address)
	p.word(andmask)
	p.word(ormask)
	tx := pdu{0x16, p.payload()}
	ret := &X16xMaskWriteHolding{}
	decode := func(r *dataReader) error {
		if len(r.data) != 6 {
			return fmt.Errorf("Expect Mask Holding Register response to be exactly 6 chars, not %v", len(r.data))
		}
		got, err := r.word()
		if err != nil {
			return err
		}
		if got != address {
			return fmt.Errorf("Expect Mask Holding Register response to be for the same address %v, not %v", address, got)
		}

		amask, err := r.word()
		if err != nil {
			return err
		}
		if amask != andmask {
			return fmt.Errorf("Expect Mask Holding Register response to be for the same AND mask %v, not %v", andmask, amask)
		}

		omask, err := r.word()
		if err != nil {
			return err
		}
		if amask != andmask {
			return fmt.Errorf("Expect Mask Holding Register response to be for the same OR mask %v, not %v", ormask, omask)
		}

		ret.Address = address
		ret.ANDMask = andmask
		ret.ORMask = ormask
		return nil
	}
	err := <-c.query(tout, tx, decode)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// X18xReadFIFOQueue server response to a Read FIFO Queue request
type X18xReadFIFOQueue struct {
	Address int
	Values  []int
}

func (s X18xReadFIFOQueue) String() string {
	cnt := len(s.Values)
	txt := make([]string, cnt)
	for i, v := range s.Values {
		txt[i] = fmt.Sprintf("    0x%04x:   0x%04x  % 6d\n", s.Address+i, v, v)
	}
	return fmt.Sprintf("X18xReadFIFOQueue %05d -> %05d (count %v)\n", s.Address, s.Address+cnt-1, cnt) + strings.Join(txt, "")
}

func (c client) ReadFIFOQueue(from int, tout time.Duration) (*X18xReadFIFOQueue, error) {
	p := dataBuilder{}
	p.word(from)
	tx := pdu{0x18, p.payload()}

	ret := &X18xReadFIFOQueue{}
	decode := func(r *dataReader) error {
		sz, err := r.word()
		if err != nil {
			return err
		}
		if len(r.data) != int(sz+2) {
			return fmt.Errorf("Expect Read FIFO Queue response to have correct internal length, %v not %v", len(r.data)-2, sz)
		}

		count, err := r.word()
		if err != nil {
			return err
		}
		if count*2+2 != sz {
			return fmt.Errorf("Expect Read FIFO Queue response to have corroborating internal count and length, %v*2 + 2 != %v", count, sz)
		}
		v, err := r.words(count)
		if err != nil {
			return err
		}

		ret.Address = from
		ret.Values = v
		return nil
	}
	err := <-c.query(tout, tx, decode)
	if err != nil {
		return nil, err
	}
	return ret, nil
}
