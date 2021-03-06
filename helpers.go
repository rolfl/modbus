package modbus

/*
this file contains some utility functions
*/

import "fmt"

func wordClamp(val int) int {
	if val < 0 {
		return 0
	}
	if val > 65535 {
		return 65535
	}
	return val
}

func byteClamp(val int) int {
	if val < 0 {
		return 0
	}
	if val > 255 {
		return 255
	}
	return val
}

func checkPanic(to string, val int, max int) {
	if val < 0 {
		panic(fmt.Sprintf("Unable to convert %v to %v - negative", val, to))
	}
	if val > max {
		panic(fmt.Sprintf("Unable to convert %v to %v - exceeds max value %v", val, to, max))
	}
}

func wordPanic(val int) uint16 {
	checkPanic("uint16", val, 65535)
	return uint16(val)
}

func bytePanic(val int) byte {
	checkPanic("byte", val, 255)
	return byte(val)
}

// getWord retrieves a 16-bit word in standard Modbus layout (bigendian) from a byte slice.
func getWord(data []byte, index int) uint16 {
	return uint16(data[index])<<8 | uint16(data[index+1])
}

// iGetWord retrieves a 16-bit word in standard Modbus layout (bigendian) from a byte slice.
// Returns the value as an int instead of a byte (reduces casting in some use cases)
func iGetWord(data []byte, index int) int {
	return int(getWord(data, index))
}

// setWord sets a 16-bit word in standard Modbus layout (bigendian) in a byte slice.
func setWord(data []byte, index int, value uint16) {
	data[index] = byte(value >> 8)
	data[index+1] = byte(value & 0xFF)
}

// iSetWord sets a 16-bit word in standard Modbus layout (bigendian) in a byte slice.
// The value is supplied as an int for convenience
func iSetWord(data []byte, index int, value int) {
	setWord(data, index, wordPanic(value))
}

// getWordLE retrieves a 16-bit word in Little-endian layout (only used for CRC) from a byte slice.
func getWordLE(data []byte, index int) (word uint16) {
	word = uint16(data[index]) | uint16(data[index+1])<<8
	return
}

// setWordLE sets a 16-bit word in standard Little-endian layout (only uised for CRC) in a byte slice.
func setWordLE(data []byte, index int, value uint16) {
	data[index] = byte(value & 0xFF)
	data[index+1] = byte(value >> 8)
}

// getByte retrieves an 8-bit word in standard Modbus layout from a byte slice.
// This is nothing more than data[index] but it provides consistency with GetWord
func getByte(data []byte, index int) (byt byte) {
	byt = data[index]
	return
}

// iGetByte retrieves an 8-bit word in standard Modbus layout from a byte slice.
// This is nothing more than data[index] but it provides consistency with GetWord
// Returns the value as an int instead of a byte (reduces casting in some use cases)
func iGetByte(data []byte, index int) int {
	return int(getByte(data, index))
}

// setByte sets an 8-bit value in standard Modbus layout in a byte slice.
// This is nothing more than data[index] = value but it provides consistency with SetWord
func setByte(data []byte, index int, value byte) {
	data[index] = value
}

// iSetByte sets an 8-bit value in standard Modbus layout in a byte slice.
// This is nothing more than data[index] = value but it provides consistency with SetWord
func iSetByte(data []byte, index int, value int) {
	data[index] = bytePanic(value)
}

func computeCRC16(data []byte) (crc uint16) {
	crc = 0xFFFF
	for _, d := range data {
		crc ^= uint16(d)
		for b := 0; b < 8; b++ {
			if crc&0x1 == 1 {
				crc >>= 1
				crc ^= 0xA001
			} else {
				crc >>= 1
			}
		}
	}
	return
}

// serverCheckAddress validates that an address and length is covered by the available data
func serverCheckAddress(name string, address, count, limit int) error {
	if address+count <= limit {
		return nil
	}
	plural := "s"
	if count == 1 {
		plural = ""
	}
	return IllegalAddressErrorF("%v: unable to get %v item%v from %v with limit of %v", name, count, plural, address, limit)
}
