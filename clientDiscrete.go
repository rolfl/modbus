package modbus

import (
	"fmt"
	"strings"
	"time"
)

// X02xReadDiscretes contains the results of reading discretes from a remote server
type X02xReadDiscretes struct {
	Address   int
	Discretes []bool
}

func (s X02xReadDiscretes) String() string {
	parts := make([]string, 0, len(s.Discretes))
	for i, v := range s.Discretes {
		d := '-'
		if v {
			d = '#'
		}
		parts = append(parts, fmt.Sprintf("      %05d: %c\n", s.Address+i, d))
	}
	return fmt.Sprintf("X02xReadDiscretes\n%v", strings.Join(parts, ""))
}

func (c *client) ReadDiscretes(from int, count int, tout time.Duration) (*X02xReadDiscretes, error) {
	p := dataBuilder{}
	p.word(from)
	p.word(count)
	tx := pdu{0x02, p.payload()}
	ret := &X02xReadDiscretes{}
	decode := func(r *dataReader) error {
		bools, err := r.bits(count)
		if err != nil {
			return err
		}
		ret.Address = from
		ret.Discretes = bools

		return nil
	}
	err := <-c.query(tout, tx, decode)
	if err != nil {
		return nil, err
	}
	return ret, nil
}
