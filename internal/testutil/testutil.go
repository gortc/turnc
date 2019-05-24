package testutil

import (
	"testing"

	"github.com/pion/logging"
)

// EnsureNoErrors errors if logs contain any error message.
func EnsureNoErrors(t *testing.T, logs *Observer) {
	t.Helper()
	logs.Lock()
	defer logs.Unlock()
	for _, e := range logs.Messages {
		if e.Level == logging.LogLevelError {
			t.Error(e.Message)
		}
	}
}
