package modbus

import (
	"fmt"
	"sort"
	"strings"
	"time"
)

// X07xReadExceptionStatus server response to a ServerID function request
type X07xReadExceptionStatus struct {
	ExceptionStatus int
}

func (s X07xReadExceptionStatus) String() string {
	return fmt.Sprintf("X07xReadExceptionStatus %08b", s.ExceptionStatus)
}

func (c *client) ReadExceptionStatus(tout time.Duration) (*X07xReadExceptionStatus, error) {
	tx := pdu{function: 0x07, data: make([]uint8, 0)}
	ret := &X07xReadExceptionStatus{}
	decode := func(r *dataReader) error {
		s, err := r.byte()
		if err != nil {
			return err
		}
		ret.ExceptionStatus = s
		return nil
	}
	err := <-c.query(tout, tx, decode)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// X11xServerID server response to a ServerID function request
type X11xServerID struct {
	ServerID     []byte
	RunIndicator bool
}

func (s X11xServerID) String() string {
	parts := make([]string, 0, len(s.ServerID))
	for _, p := range s.ServerID {
		parts = append(parts, fmt.Sprintf("%02x", p))
	}
	return fmt.Sprintf("X11xServerID %v (%q) Running %v", strings.Join(parts, " "), s.ServerID, s.RunIndicator)
}

func (c *client) ServerID(tout time.Duration) (*X11xServerID, error) {
	tx := pdu{function: 0x11, data: make([]uint8, 0)}
	ret := &X11xServerID{}
	decode := func(r *dataReader) error {
		sz, err := r.byte()
		if err != nil {
			return err
		}
		data, err := r.bytesRaw(sz)
		if err != nil {
			return err
		}
		end := len(data) - 1
		ri := data[end] > 0
		sid := data[:end]

		ret.ServerID = sid
		ret.RunIndicator = ri
		return nil
	}
	err := <-c.query(tout, tx, decode)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// X2BxDeviceIdentification server response to a Device Identification function request
type X2BxDeviceIdentification struct {
	VendorName          string
	ProductCode         string
	MajorMinorVersion   string
	VendorURL           string
	ProductName         string
	ModelName           string
	UserApplicationName string
	Additional          []string
}

func (s X2BxDeviceIdentification) String() string {
	parts := make([]string, 8)
	parts[0] = "X2BxDeviceIdentification"
	parts[1] = fmt.Sprintf("Vendor Name:   %v", s.VendorName)
	parts[2] = fmt.Sprintf("Product Code:  %v", s.ProductCode)
	parts[3] = fmt.Sprintf("MajorMinor:    %v", s.MajorMinorVersion)
	parts[4] = fmt.Sprintf("Vendor URL:    %v", s.VendorURL)
	parts[5] = fmt.Sprintf("Product Name:  %v", s.ProductName)
	parts[6] = fmt.Sprintf("Model Name:    %v", s.ModelName)
	parts[7] = fmt.Sprintf("User App Name: %v", s.UserApplicationName)
	for i, v := range s.Additional {
		parts = append(parts, fmt.Sprintf("Optional 0x%02x: %v", i+0x80, v))
	}
	return strings.Join(parts, "\n      ")
}

type devInfoAccumulator struct {
	init     bool
	code     int
	conforms int
	more     bool
	next     int
	objects  map[int]string
}

func getMoreDeviceID(c *client, fill *devInfoAccumulator, tout time.Duration) error {
	p := dataBuilder{}
	p.byte(0x0e) // MEI type 14.
	p.byte(fill.code)
	p.byte(fill.next)
	tx := pdu{0x2b, p.payload()}

	decode := func(r *dataReader) error {
		if len(r.data) < 6 {
			return fmt.Errorf("MoreDeviceId requires at least 6 bytes of content, not %v", len(r.data))
		}
		mei, _ := r.byte()
		code, _ := r.byte()
		cnf, _ := r.byte()
		more, _ := r.byte()
		next, _ := r.byte()
		count, _ := r.byte()

		if mei != 0x0E || code != fill.code {
			return fmt.Errorf("Expect DeviceIdentification response to have MEI and code, %v and %v not %v and %v", 0x0e, fill.code, mei, code)
		}

		if !fill.init {
			fill.init = true
			fill.conforms = cnf
		}

		fill.next = next
		fill.more = more != 0

		// OK, expect items now, need to loop.
		for i := 0; i < count; i++ {
			oid, err := r.byte()
			if err != nil {
				return err
			}
			olen, err := r.byte()
			if err != nil {
				return err
			}
			obytes, err := r.bytesRaw(olen)
			if err != nil {
				return err
			}
			fill.objects[oid] = string(obytes)
		}

		return nil
	}
	err := <-c.query(tout, tx, decode)
	return err
}

func getSection(c *client, sect int, fill *devInfoAccumulator, tout time.Duration) error {
	from := 0
	switch sect {
	case 1:
		from = 0
	case 2:
		from = 3
	case 3:
		from = 0x80
	}
	fill.code = sect
	fill.next = from
	fill.more = true
	for fill.more {
		err := getMoreDeviceID(c, fill, tout)
		if err != nil {
			return err
		}
	}
	return nil
}

func (c *client) DeviceIdentification(tout time.Duration) (*X2BxDeviceIdentification, error) {
	// initially just basics, we update that later....
	fill := &devInfoAccumulator{objects: make(map[int]string), conforms: 0x01}
	err := getSection(c, 1, fill, tout)
	if err != nil {
		return nil, err
	}
	if (fill.conforms & 0x7f) >= 2 {
		err = getSection(c, 2, fill, tout)
		if err != nil {
			return nil, err
		}
	}
	if (fill.conforms & 0x7f) >= 3 {
		err = getSection(c, 3, fill, tout)
		if err != nil {
			return nil, err
		}
	}

	ret := &X2BxDeviceIdentification{}
	ret.VendorName = fill.objects[0]
	ret.ProductCode = fill.objects[1]
	ret.MajorMinorVersion = fill.objects[2]
	ret.VendorURL = fill.objects[3]
	ret.ProductName = fill.objects[4]
	ret.ModelName = fill.objects[5]
	ret.UserApplicationName = fill.objects[6]
	keys := make([]int, 0, len(fill.objects))
	for k := range fill.objects {
		if k >= 0x80 {
			keys = append(keys, int(k))
		}
	}
	sort.Ints(keys)
	ret.Additional = make([]string, len(keys))
	for i, k := range keys {
		ret.Additional[i] = fill.objects[k]
	}
	return ret, nil
}

var identifications = []string{"Vendor Name", "Product Code", "Major Minor Version", "Vendor URL", "Product Name", "Model Name", "User Application Name"}

// X2BxDeviceIdentificationObject server response to a Device Identification function request for a single Object
type X2BxDeviceIdentificationObject struct {
	ObjectID int
	Name     string
	Value    string
}

func (s X2BxDeviceIdentificationObject) String() string {
	return fmt.Sprintf("X2BxDeviceIdentificationObject %v (0x%02x): '%v'", s.Name, s.ObjectID, s.Value)
}

func (c *client) DeviceIdentificationObject(objectID int, tout time.Duration) (*X2BxDeviceIdentificationObject, error) {
	p := dataBuilder{}
	p.byte(0x0e)
	p.byte(4)
	p.byte(objectID)
	tx := pdu{0x2b, p.payload()}

	ret := &X2BxDeviceIdentificationObject{}
	ret.ObjectID = objectID
	if objectID < 0x07 {
		ret.Name = identifications[objectID]
	} else if objectID >= 0x80 {
		ret.Name = fmt.Sprintf("Extended 0x%02x", objectID)
	} else {
		return nil, fmt.Errorf("Illegal Object ID 0x%02x", objectID)
	}

	decode := func(r *dataReader) error {
		if len(r.data) < 6 {
			return fmt.Errorf("Expect DeviceIdentification response to be at least 6 chars, not %v", len(r.data))
		}
		mei, _ := r.byte()
		code, _ := r.byte()
		// cnf := rx.data[2]
		r.byte()
		more, _ := r.byte()
		next, _ := r.byte()
		count, _ := r.byte()
		if mei != 0x0E || code != 4 || count != 1 {
			return fmt.Errorf("Expect DeviceIdentification response to have MEI, code and count, %v, %v and %v not %v, %v and %v", 0x0e, 4, 1, mei, code, count)
		}
		if next != 0x00 || more != 0x00 {
			return fmt.Errorf("Expect DeviceIdentificationObject response to have more-follows and next %v and %v not %v and %v", 0x00, 0x00, more, next)
		}
		oid, _ := r.byte()
		if oid != objectID {
			return fmt.Errorf("Expect DeviceIdentificationObject response to have objectId %v not %v", objectID, oid)
		}
		olen, _ := r.byte()
		sbytes, err := r.bytesRaw(olen)
		if err != nil {
			return nil
		}
		ret.Value = string(sbytes)
		return nil
	}
	err := <-c.query(tout, tx, decode)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// X0BxCommEventCounter server response to a Comm Event Counter function request
type X0BxCommEventCounter struct {
	Busy       bool
	EventCount int
}

func (s X0BxCommEventCounter) String() string {
	return fmt.Sprintf("X0BxCommEventCounter busy %v -> count %v", s.Busy, s.EventCount)
}

func (c *client) CommEventCounter(tout time.Duration) (*X0BxCommEventCounter, error) {
	tx := pdu{function: 0x0B, data: make([]uint8, 0)}
	ret := &X0BxCommEventCounter{}
	decode := func(r *dataReader) error {
		busy, err := r.word()
		if err != nil {
			return err
		}
		ec, err := r.word()
		if err != nil {
			return err
		}
		ret.Busy = busy == 0xFFFF
		ret.EventCount = ec
		return nil
	}
	err := <-c.query(tout, tx, decode)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// X0CxCommEventLog server response to a Comm Event Counter function request
type X0CxCommEventLog struct {
	Busy         bool
	EventCount   int
	MessageCount int
	Events       []int
}

func (s X0CxCommEventLog) String() string {
	logs := make([]string, len(s.Events))
	for i, e := range s.Events {
		msg := make([]string, 0, 5)
		msg = append(msg, fmt.Sprintf("      %08b", e))
		if e&0x80 != 0 {
			// Receive event
			msg = append(msg, "<---RX")
			if e&0x40 != 0 {
				msg = append(msg, "BC")
			}
			if e&0x20 != 0 {
				msg = append(msg, "LOM")
			}
			e &= 0x1f
			if e != 0 {
				msg = append(msg, ">>FAIL<<")
				if e&0x10 != 0 {
					msg = append(msg, "OR")
				}
				if e&0x02 != 0 {
					msg = append(msg, "CE")
				}
			} else {
				msg = append(msg, "OK")
			}
		} else if e&0x40 != 0 {
			// Send event
			msg = append(msg, "TX--->")
			if e&0x20 != 0 {
				msg = append(msg, "LOM")
			}
			e &= 0x1f
			if e != 0 {
				msg = append(msg, ">>FAIL<<")
				if e&0x10 != 0 {
					msg = append(msg, "TO")
				}
				if e&0x08 != 0 {
					msg = append(msg, "NAK")
				}
				if e&0x04 != 0 {
					msg = append(msg, "BSY")
				}
				if e&0x02 != 0 {
					msg = append(msg, "AB")
				}
				if e&0x01 != 0 {
					msg = append(msg, "RE")
				}
			} else {
				msg = append(msg, "OK")
			}
		} else if e == 0x40 {
			msg = append(msg, ">>LOM<<")
		} else if e == 0x00 {
			msg = append(msg, ">>START<<")
		} else {
			msg = append(msg, "**UNKNOWN**")
		}
		logs[i] = strings.Join(msg, " ")
	}
	return fmt.Sprintf("X0CxCommEventLog busy %v -> events %v -> messages %v\n%v", s.Busy, s.EventCount, s.MessageCount, strings.Join(logs, "\n"))
}

func (c *client) CommEventLog(tout time.Duration) (*X0CxCommEventLog, error) {
	tx := pdu{function: 0x0C, data: make([]uint8, 0)}
	ret := &X0CxCommEventLog{}
	decode := func(r *dataReader) error {
		len, err := r.byte()
		if err != nil {
			return err
		}
		stat, err := r.word()
		if err != nil {
			return err
		}
		ec, err := r.word()
		if err != nil {
			return err
		}
		mc, err := r.word()
		if err != nil {
			return err
		}
		events, err := r.bytes(len - 6)
		if err != nil {
			return err
		}

		ret.Busy = stat == 0xffff
		ret.EventCount = ec
		ret.MessageCount = mc
		ret.Events = events
		return nil
	}
	err := <-c.query(tout, tx, decode)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// X08xDiagnosticEcho server response to a Diagnostic Return Query data function request
type X08xDiagnosticEcho struct {
	data []int
}

func (s X08xDiagnosticEcho) String() string {
	str := make([]string, len(s.data))
	for i, v := range s.data {
		str[i] = fmt.Sprintf("%04x", v)
	}
	return fmt.Sprintf("X08xDiagnosticEcho (words %v bytes %v) %v", len(s.data), len(s.data)*2, str)
}

func (c *client) DiagnosticEcho(data []int, tout time.Duration) (*X08xDiagnosticEcho, error) {
	sz := len(data)*2 + 2
	tx := pdu{function: 0x08, data: make([]uint8, sz)}
	setWord(tx.data, 0, 0) // 0x00 subfunction
	for i, v := range data {
		iSetWord(tx.data, 2+i*2, v)
	}
	ret := &X08xDiagnosticEcho{}
	decode := func(r *dataReader) error {
		cnt := len(r.data) / 2
		got, err := r.words(cnt)
		if err != nil {
			return err
		}
		if len(got) != len(data)+1 {
			return fmt.Errorf("Expect DiagnosticEcho response to be exactly %v words, not %v", len(data)+1, len(got))
		}
		if got[0] != 0 {
			return fmt.Errorf("Expect DiagnosticEcho response to be for the subfunction 0x0000, not 0x%04x", got[0])
		}
		got = got[1:]
		for i, v := range data {
			if got[i] != int(v) {
				return fmt.Errorf("Expect DiagnosticEcho response to match, but at index %v got 0x%04x but expected 0x%04x", i, got[i], v)
			}
		}
		ret.data = got
		return nil
	}
	err := <-c.query(tout, tx, decode)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// X08xDiagnosticRegister server response to a Diagnostic Return Query data function request
type X08xDiagnosticRegister struct {
	Register int
}

func (s X08xDiagnosticRegister) String() string {
	return fmt.Sprintf("X08xDiagnosticRegister 0x%04x", s.Register)
}

func (c *client) DiagnosticRegister(tout time.Duration) (*X08xDiagnosticRegister, error) {
	tx := pdu{function: 0x08, data: make([]uint8, 4)}
	setWord(tx.data, 0, 2) // 0x02 subfunction
	setWord(tx.data, 2, 0) // 0x00 subfunction
	ret := &X08xDiagnosticRegister{}
	decode := func(r *dataReader) error {
		if len(r.data) != 4 {
			return fmt.Errorf("Expect DiagnosticEcho response to be exactly 4 bytes, not %v", len(r.data))
		}
		sf, _ := r.word()
		reg, _ := r.word()
		if sf != 2 {
			return fmt.Errorf("Expect DiagnosticEcho response to be for the subfunction 0x0002, not 0x%04x", sf)
		}
		ret.Register = reg
		return nil
	}
	err := <-c.query(tout, tx, decode)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

func (c *client) DiagnosticClear(tout time.Duration) error {
	p := dataBuilder{}
	p.word(0x0a)
	p.word(0x00)
	tx := pdu{0x08, p.payload()}
	decode := func(r *dataReader) error {
		if len(r.data) != 4 {
			return fmt.Errorf("Expect DiagnosticClear response to be exactly 4 bytes, not %v", len(r.data))
		}
		sf, _ := r.word()
		ec, _ := r.word()
		if sf != 0x0a {
			return fmt.Errorf("Expect DiagnosticClear response to be for the subfunction 0x000a, not 0x%04x", sf)
		}
		if ec != 0 {
			return fmt.Errorf("Expect DiagnosticClear response to echo 0x00 but got  0x%04x", ec)
		}
		return nil
	}
	err := <-c.query(tout, tx, decode)
	if err != nil {
		return err
	}
	return nil
}

// Diagnostic is a type used to identify counters in the modbus diagnostics in client.DiagnosticCount(...)
type Diagnostic uint16

// Diagnostic constants for querying various counters on a Modbus Server
const (
	BusMessages Diagnostic = 0x0B + iota
	BusCommErrors
	BusExceptionErrors
	ServerMessages
	ServerNoResponses
	ServerNAKs
	ServerBusies
	BusCharacterOverruns
)

var diagNames = [8]string{"BusMessages", "BusCommErrors", "BusExceptionErrors", "ServerMessages", "ServerNoResponses", "ServerNAKs", "ServerBusies", "BusCharacterOverruns"}

func (d Diagnostic) String() string {
	idx := int(d) - int(BusMessages)
	if idx < 0 || idx >= len(diagNames) {
		return fmt.Sprintf("UnknownDiagnostic 0x%04x", uint16(d))
	}
	return diagNames[idx]
}

// X08xDiagnosticCount server response to a Diagnostic Counter function request
type X08xDiagnosticCount struct {
	Counter Diagnostic
	Count   int
}

func (s X08xDiagnosticCount) String() string {
	return fmt.Sprintf("X08xDiagnosticCount %v -> %v", s.Counter, s.Count)
}

func (c *client) DiagnosticCount(counter Diagnostic, tout time.Duration) (*X08xDiagnosticCount, error) {
	p := dataBuilder{}
	p.word(int(counter)) // first word, the counter to get
	p.word(0)            // second word, the "data field" is set to zero. The response data field will be the value.
	tx := pdu{0x08, p.payload()}

	ret := &X08xDiagnosticCount{}
	decode := func(r *dataReader) error {
		if len(r.data) != 4 {
			return fmt.Errorf("Expect Diagnostic Count response to be exactly 4 bytes, not %v", len(r.data))
		}
		ck, _ := r.word()
		cnt, _ := r.word()
		check := Diagnostic(ck)
		if check != counter {
			return fmt.Errorf("Expect Diagnostic Count response to be for the same counter %v not %v", counter, check)
		}
		ret.Counter = counter
		ret.Count = cnt
		return nil
	}
	err := <-c.query(tout, tx, decode)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// X08xDiagnosticOverrunClear server response to a Diagnostic Overrun Clear data function request
type X08xDiagnosticOverrunClear struct {
	Echo int
}

func (s X08xDiagnosticOverrunClear) String() string {
	return fmt.Sprintf("X08xDiagnosticOverrunClear 0x%04x", s.Echo)
}

func (c *client) DiagnosticOverrunClear(echo int, tout time.Duration) (*X08xDiagnosticOverrunClear, error) {
	p := dataBuilder{}
	p.word(0x14) // 0x14 subfunction
	p.word(echo) // ???
	tx := pdu{0x08, p.payload()}
	ret := &X08xDiagnosticOverrunClear{}
	decode := func(r *dataReader) error {
		if len(r.data) != 4 {
			return fmt.Errorf("Expect Diagnostic Overrun Clear response to be exactly 4 bytes, not %v", len(r.data))
		}
		sf, _ := r.word()
		ec, _ := r.word()
		if sf != 0x0a {
			return fmt.Errorf("Expect DiagnosticClear response to be for the subfunction 0x000a, not 0x%04x", sf)
		}
		if ec != echo {
			return fmt.Errorf("Expect DiagnosticClear response to echo 0x%04x but got  0x%04x", echo, ec)
		}
		ret.Echo = ec
		return nil
	}
	err := <-c.query(tout, tx, decode)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

/*
// X00xDebugRaw server response to a Diagnostic Overrun Clear data function request
type X00xDebugRaw struct {
	function uint8
	data     []uint8
}

func toHex(src []uint8) string {
	out := make([]string, len(src))
	for i, val := range src {
		out[i] = fmt.Sprintf("%02x", val)
	}
	return strings.Join(out, " ")
}

func (s X00xDebugRaw) String() string {
	src := s.data[:]
	out := make([]string, 0)
	offset := 0
	for len(src) > 16 {
		sub := src[:16]
		src = src[16:]
		out = append(out, fmt.Sprintf("   0x%02x -: %v", offset, toHex(sub)))
		offset += 16
	}
	out = append(out, fmt.Sprintf("   0x%02x -: %v", offset, toHex(src)))
	return fmt.Sprintf("X00xDebugRaw function 0x%02x Response length %v\n%v", s.function, len(s.data), strings.Join(out, "\n"))
}

func (c *client) DebugRaw(function uint8, payload []uint8, tout time.Duration) (*X00xDebugRaw, error) {
	tx := pdu{function: function, data: payload}
	ret := &X00xDebugRaw{}
	decode := func(rx pdu) error {
		ret.data = make([]uint8, len(rx.data))
		copy(ret.data, rx.data)
		return nil
	}
	err := <-c.query(tout, tx, decode)
	if err != nil {
		return nil, err
	}
	return ret, nil
}
*/
