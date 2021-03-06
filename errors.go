package modbus

import (
	"fmt"
)

// Error is a custom type for Modbus errors
type Error struct {
	msg  string
	code uint8
}

func (err *Error) Error() string {
	return err.msg
}

// Code is the Modbus code used to identify the type of modbus error
func (err *Error) Code() uint8 {
	return err.code
}

// PDU Returns the error in the form of a Modbus exception response PDU
func (err *Error) asPDU(function uint8) pdu {
	p := pdu{}
	p.function |= 0x80
	p.data = make([]uint8, 1)
	p.data[0] = err.code
	return p
}

// IllegalFunctionErrorF represents an invalid function code - Modbus error code 1
func IllegalFunctionErrorF(format string, args ...interface{}) *Error {
	return &Error{fmt.Sprintf(format, args...), 1}
}

// IllegalAddressErrorF represents an invalid address - Modbus error code 2
func IllegalAddressErrorF(format string, args ...interface{}) *Error {
	return &Error{fmt.Sprintf(format, args...), 2}
}

// IllegalValueErrorF represents an illegal data value - Modbus error code 3
func IllegalValueErrorF(format string, args ...interface{}) *Error {
	return &Error{fmt.Sprintf(format, args...), 3}
}

// ServerFailureErrorF represents an error that is not represented by the above types  - Modbus error code 4
func ServerFailureErrorF(format string, args ...interface{}) *Error {
	return &Error{fmt.Sprintf(format, args...), 4}
}

// ServerBusyErrorF represents a condition in which the server is busy and cannot process the client request  - Modbus error code 6
func ServerBusyErrorF(format string, args ...interface{}) *Error {
	return &Error{fmt.Sprintf(format, args...), 6}
}
