package modbus

/*
This file contains the routines for reading from and writing to PDU frames
*/

import "fmt"

// dataBuilder is used to build outgoing frames we send to a remote system
type dataBuilder struct {
	sizes []int
	data  []byte
}

func intsToBytes(ints []int) []byte {
	b := make([]byte, len(ints))
	for i, v := range ints {
		b[i] = bytePanic(v)
	}
	return b
}

func bytesToInt(bytes []byte) []int {
	ints := make([]int, len(bytes))
	for i, v := range bytes {
		ints[i] = int(v)
	}
	return ints
}

func intsToWords(ints []int) []uint16 {
	w := make([]uint16, len(ints))
	for i, v := range ints {
		w[i] = wordPanic(v)
	}
	return w
}

func (p *dataBuilder) payload() []byte {
	for _, pos := range p.sizes {
		p.data[pos] = bytePanic(len(p.data) - pos - 1)
	}
	return p.data
}

func (p *dataBuilder) byte(b int) {
	p.data = append(p.data, bytePanic(b))
}

func (p *dataBuilder) bytes(s ...int) {
	p.data = append(p.data, intsToBytes(s)...)
}

func (p *dataBuilder) nbytes(s ...int) {
	p.byte(len(s))
	p.data = append(p.data, intsToBytes(s)...)
}

func (p *dataBuilder) word(w int) {
	wordPanic(w)
	p.data = append(p.data, byte(w>>8), byte(w&0xff))
}

func (p *dataBuilder) words(wds ...int) {
	for _, w := range wds {
		p.word(w)
	}
}
func (p *dataBuilder) nwords(s ...int) {
	p.byte(len(s))
	p.words(s...)
}

func (p *dataBuilder) bits(bits ...bool) {
	bytes := (len(bits) + 7) / 8
	packed := make([]int, bytes)
	for c, v := range bits {
		i := (c / 8)
		b := (c % 8)
		if v {
			packed[i] |= (1 << b)
		}
	}
	p.nbytes(packed...)
}

func (p *dataBuilder) nbits(bits ...bool) {
	// always count, then byte count, then packed bits.
	p.word(len(bits))
	p.bits(bits...)
}

func (p *dataBuilder) beacon() {
	p.sizes = append(p.sizes, len(p.data))
	p.byte(0)
}

type dataReader struct {
	cursor int
	data   []byte
}

func getReader(payload []byte) dataReader {
	return dataReader{0, payload}
}

func (p *dataReader) canRead(count int) error {
	over := p.cursor + count - len(p.data)
	if over > 0 {
		os := ""
		if over > 1 {
			os = "s"
		}
		cs := ""
		if count > 1 {
			cs = "s"
		}
		return fmt.Errorf("Unable to read %v byte%v beyond end of data. Request %v byte%v from %v in %v size slice", over, os, count, cs, p.cursor, len(p.data))
	}
	return nil
}

func (p *dataReader) byte() (int, error) {
	if err := p.canRead(1); err != nil {
		return 0, err
	}
	b := p.data[p.cursor]
	p.cursor++
	return int(b), nil
}

func (p *dataReader) bytesRaw(count int) ([]byte, error) {
	if err := p.canRead(count); err != nil {
		return nil, err
	}
	ret := p.data[p.cursor : p.cursor+count]
	p.cursor += count
	return ret, nil
}

func (p *dataReader) bytes(count int) ([]int, error) {
	ret, err := p.bytesRaw(count)
	if err != nil {
		return nil, err
	}
	return bytesToInt(ret), nil
}

func (p *dataReader) nbytes() ([]int, error) {
	count, err := p.byte()
	if err != nil {
		return nil, err
	}
	return p.bytes(count)
}

func (p *dataReader) word() (int, error) {
	if err := p.canRead(2); err != nil {
		return 0, err
	}
	w := uint16(p.data[p.cursor])<<8 | uint16(p.data[p.cursor+1])
	p.cursor += 2
	return int(w), nil
}

func (p *dataReader) words(count int) ([]int, error) {
	if err := p.canRead(count * 2); err != nil {
		return nil, err
	}
	wds := make([]int, 0, count)
	for i := 0; i < count; i++ {
		w, _ := p.word()
		wds = append(wds, w)
	}
	return wds, nil
}

func (p *dataReader) nwords() ([]int, error) {
	count, err := p.byte()
	if err != nil {
		return nil, err
	}
	return p.words(count)
}

func (p *dataReader) bits(count int) ([]bool, error) {
	packed, err := p.nbytes()
	if err != nil {
		return nil, err
	}
	x := (count + 7) / 8
	if len(packed) != x {
		return nil, fmt.Errorf("Expected %v bits to be packed in to %v bytes, but got %v", count, x, len(packed))
	}
	bits := make([]bool, count)
	for c := range bits {
		i := c / 8
		b := (c % 8)
		bits[c] = (packed[i] & (1 << b)) != 0
	}
	return bits, nil
}

func (p *dataReader) nbits() ([]bool, error) {
	// always count, then byte count, then packed bits.
	count, err := p.word()
	if err != nil {
		return nil, err
	}
	packed, err := p.nbytes()
	if err != nil {
		return nil, err
	}
	bits := make([]bool, count)
	for c := range bits {
		i := c / 8
		b := (c % 8)
		bits[c] = (packed[i] & (1 << b)) != 0
	}
	return bits, nil
}

func (p *dataReader) remaining() error {
	left := len(p.data) - p.cursor
	if left != 0 {
		ls := ""
		if left != 1 {
			ls = "s"
		}
		return fmt.Errorf("Expected to read all the payload data, but %v byte%v remain", left, ls)
	}
	return nil
}
