package turnc

import (
	"io"
	"net"
	"testing"
	"time"

	"github.com/pion/logging"

	"github.com/gortc/turnc/internal/testutil"
)

type closeFunc func() error

func (f closeFunc) Close() error {
	return f()
}

type readFunc func(buf []byte) (int, error)

func (f readFunc) Read(buf []byte) (int, error) {
	return f(buf)
}

func TestMultiplexer(t *testing.T) {
	t.Run("closeLogged", func(t *testing.T) {
		logs := &testutil.Observer{Level: logging.LogLevelError}
		closeLogged(logs, "message", closeFunc(func() error {
			return io.ErrUnexpectedEOF
		}))
		if logs.Len() < 1 {
			t.Error("no errors logged")
		}
	})
	t.Run("discardLogged", func(t *testing.T) {
		logs := &testutil.Observer{Level: logging.LogLevelError}
		discardLogged(logs, "message", readFunc(func(buf []byte) (int, error) {
			return 0, io.ErrUnexpectedEOF
		}))
		if logs.Len() < 1 {
			t.Error("no errors logged")
		}
	})
	t.Run("AppData", func(t *testing.T) {
		logs := &testutil.Observer{Level: logging.LogLevelError}
		connL, connR := net.Pipe()
		m := newMultiplexer(connR, logs)
		go func() {
			if err := connL.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
				t.Error(err)
			}
			if _, err := connL.Write([]byte{1, 2, 3, 4}); err != nil {
				t.Error(err)
			}
		}()
		buf := make([]byte, 1024)
		if _, err := m.dataL.Read(buf); err != nil {
			t.Error(err)
		}
		if logs.Len() > 0 {
			t.Error("no logs expected")
		}
	})
	t.Run("Write error", func(t *testing.T) {
		logs := &testutil.Observer{Level: logging.LogLevelWarn}
		connL, connR := net.Pipe()
		m := newMultiplexer(connR, logs)
		if err := m.dataR.Close(); err != nil {
			t.Error(err)
		}
		if err := m.dataL.Close(); err != nil {
			t.Error(err)
		}
		if err := connL.SetWriteDeadline(time.Now().Add(time.Second)); err != nil {
			t.Error(err)
		}
		if _, err := connL.Write([]byte{1, 2, 3, 4}); err != nil {
			t.Error(err)
		}
		timeout := time.Tick(time.Second * 5)
		for logs.Len() < 1 {
			select {
			case <-timeout:
				t.Error("timed out waiting for logs")
			default:
				continue
			}
		}
	})
}
