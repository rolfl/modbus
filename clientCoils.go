package modbus

import (
	"fmt"
	"strings"
	"time"
)

// X01xReadCoils contains the results of reading coils from a remote server
type X01xReadCoils struct {
	Address int
	Coils   []bool
}

func (s X01xReadCoils) String() string {
	parts := make([]string, 0, len(s.Coils))
	for i, v := range s.Coils {
		d := '-'
		if v {
			d = '#'
		}
		parts = append(parts, fmt.Sprintf("      %05d: %c\n", s.Address+i, d))
	}
	return fmt.Sprintf("X01xReadCoils from %05d count %v\n%v", s.Address, len(s.Coils), strings.Join(parts, ""))
}

func (c *client) ReadCoils(from int, count int, tout time.Duration) (*X01xReadCoils, error) {
	p := dataBuilder{}
	p.word(from)
	p.word(count)
	tx := pdu{0x01, p.payload()}
	ret := &X01xReadCoils{}
	decode := func(r *dataReader) error {
		coils, err := r.bits(count)
		if err != nil {
			return err
		}
		ret.Address = from
		ret.Coils = coils
		return nil
	}
	err := <-c.query(tout, tx, decode)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// X05xWriteSingleCoil server response to a Write Single Coil request
type X05xWriteSingleCoil struct {
	Address int
	Value   bool
}

func (s X05xWriteSingleCoil) String() string {
	v := "set / on"
	if !s.Value {
		v = "clear / off"
	}
	return fmt.Sprintf("X05xWriteSingleCoil %05d -> %v", s.Address, v)
}

func (c *client) WriteSingleCoil(address int, value bool, tout time.Duration) (*X05xWriteSingleCoil, error) {
	p := dataBuilder{}
	p.word(address)
	if value {
		p.word(0xFF00)
	} else {
		p.word(0x0000)
	}
	tx := pdu{0x05, p.payload()}
	ret := &X05xWriteSingleCoil{}
	decode := func(r *dataReader) error {
		err := r.canRead(4)
		if err != nil {
			return err
		}
		a, _ := r.word()
		v, _ := r.word()
		ret.Address = a
		ret.Value = v == 0xff00
		return nil
	}
	err := <-c.query(tout, tx, decode)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// X0FxWriteMultipleCoils server response to a Write Multiple Coil request
type X0FxWriteMultipleCoils struct {
	Address int
	Count   int
}

func (s X0FxWriteMultipleCoils) String() string {
	return fmt.Sprintf("X0FxWriteMultipleCoils %05d -> %05d (count %v)", s.Address, s.Address+s.Count-1, s.Count)
}

func (c *client) WriteMultipleCoils(address int, values []bool, tout time.Duration) (*X0FxWriteMultipleCoils, error) {
	p := dataBuilder{}
	p.word(address)
	p.nbits(values...)
	tx := pdu{0x0F, p.payload()}
	ret := &X0FxWriteMultipleCoils{}
	decode := func(r *dataReader) error {
		err := r.canRead(4)
		if err != nil {
			return err
		}
		a, _ := r.word()
		c, _ := r.word()
		ret.Address = a
		ret.Count = c
		return nil
	}
	err := <-c.query(tout, tx, decode)
	if err != nil {
		return nil, err
	}
	return ret, nil
}
