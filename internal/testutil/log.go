package testutil

import (
	"fmt"
	"sync"

	"github.com/pion/logging"
)

type LogMessage struct {
	Level   logging.LogLevel
	Message string
}

type Observer struct {
	sync.Mutex
	Level    logging.LogLevel
	Messages []LogMessage
}

func (o *Observer) All() []LogMessage {
	o.Lock()
	defer o.Unlock()
	m := make([]LogMessage, 0, len(o.Messages))
	for _, msg := range o.Messages {
		m = append(m, LogMessage{
			Level:   msg.Level,
			Message: msg.Message,
		})
	}
	return m
}

func (o *Observer) Len() int {
	o.Lock()
	defer o.Unlock()
	return len(o.Messages)
}

func (o *Observer) write(lvl logging.LogLevel, format string, args ...interface{}) {
	if o.Level.Get() < lvl {
		return
	}
	o.Lock()
	defer o.Unlock()
	o.Messages = append(o.Messages, LogMessage{
		Level:   lvl,
		Message: fmt.Sprintf(format, args...),
	})
}

func (o *Observer) Trace(msg string) { o.write(logging.LogLevelTrace, msg) }
func (o *Observer) Warn(msg string)  { o.write(logging.LogLevelWarn, msg) }
func (o *Observer) Info(msg string)  { o.write(logging.LogLevelInfo, msg) }
func (o *Observer) Error(msg string) { o.write(logging.LogLevelError, msg) }
func (o *Observer) Debug(msg string) { o.write(logging.LogLevelDebug, msg) }

func (o *Observer) Tracef(format string, args ...interface{}) {
	o.write(logging.LogLevelTrace, format, args...)
}
func (o *Observer) Debugf(format string, args ...interface{}) {
	o.write(logging.LogLevelDebug, format, args...)
}
func (o *Observer) Infof(format string, args ...interface{}) {
	o.write(logging.LogLevelInfo, format, args...)
}
func (o *Observer) Warnf(format string, args ...interface{}) {
	o.write(logging.LogLevelWarn, format, args...)
}
func (o *Observer) Errorf(format string, args ...interface{}) {
	o.write(logging.LogLevelError, format, args...)
}
