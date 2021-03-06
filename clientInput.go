package modbus

import (
	"fmt"
	"strings"
	"time"
)

// X04xReadInputs server response to a Read Multiple Inputs request
type X04xReadInputs struct {
	Address int
	Values  []int
}

func (s X04xReadInputs) String() string {
	cnt := len(s.Values)
	txt := make([]string, cnt)
	for i, v := range s.Values {
		txt[i] = fmt.Sprintf("    0x%04x:   0x%04x  % 6d\n", s.Address+i, v, v)
	}
	return fmt.Sprintf("X04xReadInputs %05d -> %05d (count %v)\n", s.Address, s.Address+cnt-1, cnt) + strings.Join(txt, "")
}

func (c client) ReadInputs(from int, count int, tout time.Duration) (*X04xReadInputs, error) {
	p := dataBuilder{}
	p.word(from)
	p.word(count)
	tx := pdu{0x04, p.payload()}
	ret := &X04xReadInputs{}
	decode := func(r *dataReader) error {
		l, err := r.byte()
		if err != nil {
			return err
		}
		if len(r.data) != l+1 {
			return fmt.Errorf("Expect Read Inputs response to have correct internal length, %v not %v", len(r.data)-1, l)
		}
		if l != count*2 {
			return fmt.Errorf("Expect Read Inputs response to have correct count of values, %v not %v", count, l/2)
		}
		v, err := r.words(count)

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
