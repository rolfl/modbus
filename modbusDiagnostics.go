package modbus

/*
This file contains the storage and management go-routine for keeping track of Modbus diagnostic counts.
*/

// BusDiagnostics are values specific to the Modbus that summarize the bus status
type BusDiagnostics struct {
	// Messages represents the number of valid messages received on this Modbus
	Messages int
	// CommErrors represents the number of failed receptions (invalid CRC, etc)
	CommErrors int
	// Exceptions represents the number of fail responses this Modbus instance has sent to clients
	Exceptions int
	// Overruns represents the number of incoming requests that were larger than the max Modbus payload size
	Overruns int
}

type busDiagnosticManager struct {
	diagnostics BusDiagnostics
	operation   chan func()
	queue       int
	logCount    int
	logEntries  [64]int
}

const (
	busCommError      = 1 << 1
	busCharOverrun    = 1 << 4
	busBroadcast      = 1 << 6
	busIncoming       = 1 << 7
	busListenOnly     = 1 << 5
	busReadException  = 1 << 0
	busAbortException = 1 << 1
	busBusyException  = 1 << 2
	busNAKException   = 1 << 3
	busWriteTimeout   = 1 << 4
	busOutgoing       = 1 << 6
)

func newBusDiagnosticManager() *busDiagnosticManager {
	dm := &busDiagnosticManager{}
	dm.diagnostics = BusDiagnostics{}
	dm.operation = make(chan func(), 10)
	go dm.manager()
	return dm
}

func (bdm *busDiagnosticManager) manager() {
	for fn := range bdm.operation {
		fn()
	}
}

func (bdm *busDiagnosticManager) plog(value int) {
	bdm.logEntries[bdm.logCount%64] = value
	bdm.logCount++
}

func (bdm *busDiagnosticManager) clear() {
	got := make(chan BusDiagnostics)
	bdm.operation <- func() {
		bdm.diagnostics = BusDiagnostics{}
		bdm.logCount = 0
		close(got)
	}
	<-got
}

func (bdm *busDiagnosticManager) clearOverrun() {
	got := make(chan BusDiagnostics)
	bdm.operation <- func() {
		bdm.diagnostics.Overruns = 0
		close(got)
	}
	<-got
}

func (bdm *busDiagnosticManager) getDiagnostics() BusDiagnostics {
	got := make(chan BusDiagnostics)
	bdm.operation <- func() {
		got <- bdm.diagnostics
		close(got)
	}
	return <-got
}

func (bdm *busDiagnosticManager) message(broadcast bool) {
	done := make(chan bool)
	bdm.operation <- func() {
		bdm.diagnostics.Messages++
		bc := 0
		if broadcast {
			bc = busBroadcast
		}
		bdm.plog(busIncoming | bc)
		close(done)
	}
	<-done
}

func (bdm *busDiagnosticManager) response(p pdu) {
	done := make(chan bool)
	bdm.operation <- func() {
		log := busOutgoing
		if p.function >= 128 {
			bdm.diagnostics.Exceptions++
			code := 0
			if len(p.data) > 0 {
				code = int(p.data[0])
			}
			if code <= 3 {
				log |= busReadException
			} else if code == 4 {
				log |= busAbortException
			} else if code <= 6 {
				log |= busBusyException
			} else if code == 7 {
				log |= busNAKException
			}
		}
		bdm.plog(log)
		close(done)
	}
	<-done
}

func (bdm *busDiagnosticManager) commError() {
	done := make(chan bool)
	bdm.operation <- func() {
		bdm.diagnostics.CommErrors++
		bdm.plog(busIncoming | busCommError)
		close(done)
	}
	<-done
}

func (bdm *busDiagnosticManager) overrun() {
	done := make(chan bool)
	bdm.operation <- func() {
		bdm.diagnostics.Exceptions++
		bdm.plog(busIncoming | busCharOverrun)
		close(done)
	}
	<-done
}

func (bdm *busDiagnosticManager) logEvent(value int) {
	done := make(chan bool)
	bdm.operation <- func() {
		bdm.plog(value)
		close(done)
	}
	<-done
}

func (bdm *busDiagnosticManager) getEventLog() []int {
	done := make(chan []int)
	bdm.operation <- func() {
		count := bdm.logCount
		if count > 64 {
			count = 64
		}
		ret := make([]int, count)
		for i := range ret {
			ret[i] = bdm.logEntries[(bdm.logCount-i-1)%64]
		}
		done <- ret
		close(done)
	}
	return <-done
}
