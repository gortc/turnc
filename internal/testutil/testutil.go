package testutil

import (
	"testing"

	"go.uber.org/zap"
	"go.uber.org/zap/zaptest/observer"
)

// EnsureNoErrors errors if logs contain any error message.
func EnsureNoErrors(t *testing.T, logs *observer.ObservedLogs) {
	t.Helper()
	for _, e := range logs.TakeAll() {
		if e.Level == zap.ErrorLevel {
			t.Fatal(e)
		}
	}
}
