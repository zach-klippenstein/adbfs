package fs

import (
	"fmt"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/hanwen/go-fuse/fuse"
	"github.com/zach-klippenstein/goadb/util"
)

/*
LogEntry reports results, errors, and statistics for an individual operation.
Each method can only be called once, and will panic on subsequent calls.

If an error is reported, it is logged as a separate entry.

Example Usage

	func DoTheThing(path string) fuse.Status {
		logEntry := StartOperation("DoTheThing", path)
		defer FinishOperation(log) // Where log is a logrus logger.

		result, err := perform(path)
		if err != nil {
			logEntry.Error(err)
			return err
		}

		logEntry.Result(result)
		return logEntry.Status(fuse.OK)
	}
*/
type LogEntry struct {
	name      string
	path      string
	startTime time.Time
	err       error
	result    string
	status    string

	cacheUsed bool
	cacheHit  bool
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

// Status records the fuse.Status result of an operation.
func (r *LogEntry) Status(status fuse.Status) fuse.Status {
	if r.status != "" {
		panic(fmt.Sprintf("status already set to '%s', can't set to '%s'", r.status, status))
	}
	r.status = status.String()
	return status
}

// CacheUsed records that a cache was used to attempt to retrieve a result.
func (r *LogEntry) CacheUsed(hit bool) {
	if r.cacheUsed {
		panic(fmt.Sprintf("cache use already reported"))
	}
	r.cacheUsed = true
	r.cacheHit = hit
}

// FinishOperation should be deferred. It will log the duration of the operation, as well
// as any results and/or errors.
func (r *LogEntry) FinishOperation(log *logrus.Logger) {
	entry := log.WithFields(logrus.Fields{
		"path":        r.path,
		"duration_ms": calculateDurationMillis(r.startTime),
		"status":      r.status,
	})

	if r.result != "" {
		entry = entry.WithField("result", r.result)
	}

	if r.cacheUsed {
		entry = entry.WithField("cache_hit", r.cacheHit)
	}

	entry.Debug(r.name)

	if r.err != nil {
		log.Errorln(util.ErrorWithCauseChain(r.err))
	}
}

func calculateDurationMillis(startTime time.Time) int64 {
	return time.Now().Sub(startTime).Nanoseconds() / time.Millisecond.Nanoseconds()
}
