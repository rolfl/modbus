package modbus

// ServerDiagnostics represents a summary of the server state.
type ServerDiagnostics struct {
	Messages     int
	NoResponse   int
	ServerNAKs   int
	ServerBusy   int
	Register     int
	EventCounter int
}

type serverDiagnosticManager struct {
	diagnostics ServerDiagnostics
	operation   chan func()
	queue       int
}

func newServerDiagnosticManager() *serverDiagnosticManager {
	dm := &serverDiagnosticManager{}
	dm.diagnostics = ServerDiagnostics{}
	dm.operation = make(chan func(), 10)
	go dm.manager()
	return dm
}

func (sdm *serverDiagnosticManager) manager() {
	for fn := range sdm.operation {
		fn()
	}
}

func (sdm *serverDiagnosticManager) getDiagnostics() ServerDiagnostics {
	got := make(chan ServerDiagnostics)
	sdm.operation <- func() {
		got <- sdm.diagnostics
		close(got)
	}
	return <-got
}

func (sdm *serverDiagnosticManager) message() {
	done := make(chan bool)
	sdm.operation <- func() {
		sdm.diagnostics.Messages++
		close(done)
	}
	<-done
}

func (sdm *serverDiagnosticManager) noResponse() {
	done := make(chan bool)
	sdm.operation <- func() {
		sdm.diagnostics.NoResponse++
		close(done)
	}
	<-done
}

func (sdm *serverDiagnosticManager) serverNAKs() {
	done := make(chan bool)
	sdm.operation <- func() {
		sdm.diagnostics.ServerNAKs++
		close(done)
	}
	<-done
}

func (sdm *serverDiagnosticManager) serverBusy() {
	done := make(chan bool)
	sdm.operation <- func() {
		sdm.diagnostics.ServerBusy++
		close(done)
	}
	<-done
}

func (sdm *serverDiagnosticManager) register() {
	done := make(chan bool)
	sdm.operation <- func() {
		sdm.diagnostics.Register++
		close(done)
	}
	<-done
}

func (sdm *serverDiagnosticManager) eventCounter() {
	done := make(chan bool)
	sdm.operation <- func() {
		sdm.diagnostics.EventCounter++
		close(done)
	}
	<-done
}

func (sdm *serverDiagnosticManager) resetEventCounter() {
	done := make(chan bool)
	sdm.operation <- func() {
		sdm.diagnostics.EventCounter = 0
		close(done)
	}
	<-done
}

func (sdm *serverDiagnosticManager) eventQueued() {
	done := make(chan bool)
	sdm.operation <- func() {
		sdm.queue--
		close(done)
	}
	<-done
}

func (sdm *serverDiagnosticManager) eventComplete() {
	done := make(chan bool)
	sdm.operation <- func() {
		sdm.queue--
		close(done)
	}
	<-done
}

func (sdm *serverDiagnosticManager) busy() bool {
	done := make(chan bool)
	sdm.operation <- func() {
		done <- sdm.queue > 0
		close(done)
	}
	return <-done
}

func (sdm *serverDiagnosticManager) clear() {
	done := make(chan bool)
	sdm.operation <- func() {
		sdm.diagnostics = ServerDiagnostics{}
		close(done)
	}
	<-done
}
