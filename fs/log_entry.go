package fs

import (
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/zach-klippenstein/goadb/util"
)

/*
LogEntry reports results, errors, and statistics for an individual operation.

Example Usage

	func DoTheThing(path string) error {
		logEntry := StartOperation("DoTheThing", path)
		defer FinishOperation(log) // Where log is a logrus logger.

		result, err := perform(path)
		if err != nil {
			logEntry.Error(err)
			return err
		}

		logEntry.Result(result)
		return nil
	}
*/
type LogEntry struct {
	name      string
	path      string
	startTime time.Time
	err       error
	result    string
}

// StartOperation creates a new LogEntry with the current time.
// Should be immediately followed by a deferred call to FinishOperation.
func StartOperation(name string, path string) *LogEntry {
	return &LogEntry{
		name:      name,
		path:      path,
		startTime: time.Now(),
	}
}

// ErrorMsg records a failure result.
// Panics if called more than once.
func (r *LogEntry) ErrorMsg(err error, msg string) {
	r.Error(fmt.Errorf("%s: %v", msg, err))
}

// Error records a failure result.
// Panics if called more than once.
func (r *LogEntry) Error(err error) {
	if r.err != nil {
		panic(fmt.Sprintf("err already set to '%s', can't set to '%s'", r.err, err))
	}
	r.err = err
}

// Result records a non-failure result.
// Panics if called more than once.
func (r *LogEntry) Result(msg string, args ...interface{}) {
	result := fmt.Sprintf(msg, args...)
	if r.result != "" {
		panic(fmt.Sprintf("result already set to '%s', can't set to '%s'", r.result, result))
	}
	r.result = result
}

// FinishOperation should be deferred. It will log the duration of the operation, as well
// as any results and/or errors.
func (r *LogEntry) FinishOperation(log *logrus.Logger) {
	entry := log.WithFields(logrus.Fields{
		"path":        r.path,
		"duration_ms": calculateDurationMillis(r.startTime),
	})

	if r.result != "" {
		entry = entry.WithField("result", r.result)
	}
	entry.Debug(r.name)

	if r.err != nil {
		log.Errorln(util.ErrorWithCauseChain(r.err))
	}
}

func calculateDurationMillis(startTime time.Time) int64 {
	return time.Now().Sub(startTime).Nanoseconds() / time.Millisecond.Nanoseconds()
}
