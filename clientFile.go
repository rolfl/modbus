package modbus

import (
	"bytes"
	"fmt"
	"strings"
	"time"
)

// X14xReadRecordRequest defines a record to read from a file. To be used by ReadMultiFileRecord
type X14xReadRecordRequest struct {
	File   int
	Record int
	Length int
}

func (s X14xReadRecordRequest) String() string {
	return fmt.Sprintf("X14xReadRecordRequest 0x%04x->0x%04x: %d", s.File, s.Record, s.Length)
}

// X14xReadFileRecordResult server response to a Read Multiple File Record request
type X14xReadFileRecordResult struct {
	File   int
	Record int
	Values []int
}

func (s X14xReadFileRecordResult) String() string {
	parts := make([]string, len(s.Values))
	for i, v := range s.Values {
		parts[i] = fmt.Sprintf("      0x%04x", v)
	}
	return fmt.Sprintf("X14xReadFileRecordResult 0x%04x->0x%04x:\n%s", s.File, s.Record, strings.Join(parts, "\n"))
}

// X14xReadMultiFileRecord server response to a Read Multiple File Record request
type X14xReadMultiFileRecord struct {
	Records []X14xReadFileRecordResult
}

func (s X14xReadMultiFileRecord) String() string {
	parts := make([]string, len(s.Records))
	for i, r := range s.Records {
		parts[i] = fmt.Sprintf(" %d %v\n", i, r)
	}
	return fmt.Sprintf("X14xReadMultiFileRecord:\n%s", strings.Join(parts, "\n"))
}

func (c client) ReadMultiFileRecords(requests []X14xReadRecordRequest, tout time.Duration) (*X14xReadMultiFileRecord, error) {
	expect := 1 + len(requests)*2
	for _, r := range requests {
		expect += r.Length * 2
	}
	if expect > 253 {
		return nil, fmt.Errorf("Request will result in response of %v bytes which exceeds the limit of 253", expect)
	}
	sz := 1 + 7*len(requests)
	if sz > 253 {
		return nil, fmt.Errorf("Too many record requests since the request will be too large: %v bytes exceeds limit of 253", sz)
	}
	p := dataBuilder{}
	p.beacon() // set a byte counter here...
	for _, req := range requests {
		p.byte(6) // from spec: The reference type: 1 byte (must be specified as 6)
		p.word(req.File)
		p.word(req.Record)
		p.word(req.Length)
	}
	tx := pdu{0x14, p.payload()}
	ret := &X14xReadMultiFileRecord{Records: make([]X14xReadFileRecordResult, 0)}
	decode := func(r *dataReader) error {
		_, err := r.byte()
		if err != nil {
			return err
		}
		// We cannot use expect size to manage response, because some records may not be available (e.g. a file has just 5 records and we ask for 10)
		// if sz+1 != expect {
		// 	return fmt.Errorf("Expected Record size of %v but got %v", expect, sz+1)
		// }
		for _, req := range requests {
			rz, err := r.byte()
			if err != nil {
				return err
			}
			rcnt := rz / 2
			if rcnt > req.Length {
				return fmt.Errorf("Expected Record payload to be at most %v but got %v", req.Length*2+1, rz)
			}
			rt, err := r.byte()
			if rt != 6 {
				return fmt.Errorf("Expected reference type of 6 for all file records, not %v", rt)
			}
			wds, err := r.words(rcnt)
			if err != nil {
				return err
			}
			resp := X14xReadFileRecordResult{req.File, req.Record, wds}
			ret.Records = append(ret.Records, resp)
		}

		return nil
	}
	err := <-c.query(tout, tx, decode)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// X14xReadFileRecord server response to a Write Multiple Holding Registers request
func (c client) ReadFileRecords(file int, record int, length int, tout time.Duration) (*X14xReadFileRecordResult, error) {
	req := X14xReadRecordRequest{File: file, Record: record, Length: length}
	parm := []X14xReadRecordRequest{req}
	resp, err := c.ReadMultiFileRecords(parm, tout)
	if err != nil {
		return nil, err
	}
	if len(resp.Records) != 1 {
		return nil, fmt.Errorf("Unexpected Records result: expected 1 record but got %v", len(resp.Records))
	}
	return &resp.Records[0], nil
}

// X15xWriteFileRecordRequest data to send in a multi-file-record-write
type X15xWriteFileRecordRequest struct {
	File   int
	Record int
	Values []int
}

func (s X15xWriteFileRecordRequest) String() string {
	parts := make([]string, len(s.Values))
	for i, v := range s.Values {
		parts[i] = fmt.Sprintf("      0x%04x", v)
	}
	return fmt.Sprintf("X15xWriteFileRecordRequest 0x%04x->0x%04x:\n%s", s.File, s.Record, strings.Join(parts, "\n"))
}

// X15xWriteFileRecordResult defines the response to the WriteMultiFileRecord function for just one of the file results
type X15xWriteFileRecordResult struct {
	File   int
	Record int
	Length int
}

func (s X15xWriteFileRecordResult) String() string {
	return fmt.Sprintf("X15xWriteFileRecordResult 0x%04x->0x%04x: %d", s.File, s.Record, s.Length)
}

// X15xMultiWriteFileRecord server response to Multiple Write File Records request
type X15xMultiWriteFileRecord struct {
	Results []X15xWriteFileRecordResult
}

func (s X15xMultiWriteFileRecord) String() string {
	parts := make([]string, len(s.Results))
	for i, r := range s.Results {
		parts[i] = fmt.Sprintf(" %d %v\n", i, r)
	}
	return fmt.Sprintf("X15xMultiWriteFileRecord:\n%s", strings.Join(parts, "\n"))
}

func (c client) WriteMultiFileRecords(requests []X15xWriteFileRecordRequest, tout time.Duration) (*X15xMultiWriteFileRecord, error) {
	sz := 1 + len(requests)*7
	for _, r := range requests {
		sz += len(r.Values) * 2
	}
	if sz > 253 {
		return nil, fmt.Errorf("Request will result in a payload of %v bytes which exceeds the limit of 253", sz)
	}

	p := dataBuilder{}
	p.byte(sz - 1)

	// let's be optimistic and assume we "win" with the write, and we'll prepare the response as well.
	ret := &X15xMultiWriteFileRecord{Results: make([]X15xWriteFileRecordResult, len(requests))}
	for i, r := range requests {
		p.byte(6) // from spec: The reference type: 1 byte (must be specified as 6)
		p.word(r.File)
		p.word(r.Record)
		p.word(len(r.Values))
		p.words(r.Values...)
		ret.Results[i] = X15xWriteFileRecordResult{File: r.File, Record: r.Record, Length: len(r.Values)}
	}
	tx := pdu{0x15, p.payload()}
	decode := func(r *dataReader) error {
		r.cursor = len(r.data)
		if !bytes.Equal(tx.data, r.data) {
			return fmt.Errorf("Expect Write File Record response to be an exact echo of the request")
		}
		return nil
	}
	err := <-c.query(tout, tx, decode)
	if err != nil {
		return nil, err
	}
	return ret, nil
}

// X15xWriteFileRecord server response to a Write Multiple Holding Registers request
func (c client) WriteFileRecords(file int, record int, values []int, tout time.Duration) (*X15xWriteFileRecordResult, error) {
	rec := X15xWriteFileRecordRequest{file, record, values}
	req := []X15xWriteFileRecordRequest{rec}

	ret, err := c.WriteMultiFileRecords(req, tout)
	if err != nil {
		return nil, err
	}
	return &ret.Results[0], nil
}
