package cli

import "golang.org/x/net/trace"

// EventLog wraps trace.EventLog with logging calls to cli.Log.
type EventLog struct {
	eventLog trace.EventLog
}

func NewEventLog(family, title string) *EventLog {
	return &EventLog{trace.NewEventLog(family, title)}
}

func (l *EventLog) Debugf(format string, a ...interface{}) {
	Log.Debugf(format, a...)
	l.eventLog.Printf(format, a...)
}

func (l *EventLog) Infof(format string, a ...interface{}) {
	Log.Infof(format, a...)
	l.eventLog.Printf(format, a...)
}

func (l *EventLog) Errorf(format string, a ...interface{}) {
	Log.Errorf(format, a...)
	l.eventLog.Errorf(format, a...)
}

func (l *EventLog) Finish() {
	l.eventLog.Finish()
}
