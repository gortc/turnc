package turnc

import "testing"

func TestNopLogger(t *testing.T) {
	// Ensure at least with some confidence that logger is doing nothing.
	var log nopLogger
	if testing.AllocsPerRun(10, func() {
		log.Error("")
		log.Errorf("")
		log.Trace("")
		log.Tracef("%s %s", "foo", "bar")
		log.Warn("")
		log.Warnf("%s %s", "bar", "baz")
		log.Debug("")
		log.Debugf("")
		log.Info("")
		log.Infof("%s %d", "foo", "bar")
	}) > 0 {
		t.Error("unexpected allocations on no-op implementation")
	}
}
