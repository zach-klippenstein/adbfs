package adbfs

import (
	"fmt"
	"os"
	"syscall"
	"time"

	"github.com/Sirupsen/logrus"
	"github.com/zach-klippenstein/adbfs/internal/cli"
	"github.com/zach-klippenstein/goadb/util"
	"golang.org/x/net/trace"
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
	name       string
	hostPath   string
	devicePath string
	args       string
	startTime  time.Time
	err        error
	result     string
	status     string

	trace trace.Trace

	cacheUsed bool
	cacheHit  bool
}

var traceEntryFormatter = new(logrus.JSONFormatter)

// StartOperation creates a new LogEntry with the current time.
// Should be immediately followed by a deferred call to FinishOperation.
func StartOperation(name, hostPath string) *LogEntry {
	return &LogEntry{
		name:      name,
		hostPath:  hostPath,
		startTime: time.Now(),
		trace:     trace.New(name, hostPath),
	}
}

func (r *LogEntry) DevicePath(path string) {
	if r.devicePath != "" {
		panic(fmt.Sprintf("devicePath already set to %q can't set to %q", r.devicePath, path))
	}
	r.devicePath = path
}

func StartFileOperation(name, path, args string) *LogEntry {
	name = "File " + name
	return &LogEntry{
		name:       name,
		devicePath: path,
		args:       args,
		startTime:  time.Now(),
		trace:      trace.New(name, args),
	}
}

// ErrorMsg records a failure result.
// Panics if called more than once.
func (r *LogEntry) ErrorMsg(err error, msg string, args ...interface{}) {
	r.Error(fmt.Errorf("%s: %v", fmt.Sprintf(msg, args...), util.ErrorWithCauseChain(err)))
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
func (r *LogEntry) Status(status syscall.Errno) syscall.Errno {
	if r.status != "" {
		panic(fmt.Sprintf("status already set to '%s', can't set to '%s'", r.status, status))
	}
	if status == OK {
		r.status = "OK"
	} else {
		r.status = status.Error()
	}
	return status
}

// CacheUsed records that a cache was used to attempt to retrieve a result.
func (r *LogEntry) CacheUsed(hit bool) {
	r.cacheUsed = true
	r.cacheHit = r.cacheHit || hit
}

// FinishOperation should be deferred. It will log the duration of the operation, as well
// as any results and/or errors.
func (r *LogEntry) FinishOperation() {
	r.finishOperation(false)
}

func (r *LogEntry) SuppressFinishOperation() {
	r.finishOperation(true)
}

func (r *LogEntry) finishOperation(suppress bool) {
	entry := cli.Log.WithFields(logrus.Fields{
		"duration_ms": calculateDurationMillis(r.startTime),
		"status":      r.status,
		"pid":         os.Getpid(),
	})

	if r.devicePath != "" {
		entry = entry.WithField("path", r.devicePath)
	}
	if r.args != "" {
		entry = entry.WithField("args", r.args)
	}
	if r.result != "" {
		entry = entry.WithField("result", r.result)
	}
	if r.cacheUsed {
		entry = entry.WithField("cache_hit", r.cacheHit)
	}

	if !suppress {
		entry.Debug(r.name)
	}

	if r.err != nil {
		cli.Log.Errorln(util.ErrorWithCauseChain(r.err))
	}

	r.logTrace(entry)
}

func (r *LogEntry) logTrace(entry *logrus.Entry) {
	var msg string
	// Use a different formatter for logging to HTML trace viewer since the TextFormatter will include color escape codes.
	msgBytes, err := traceEntryFormatter.Format(entry)
	if err != nil {
		msg = fmt.Sprint(entry)
	} else {
		msg = string(msgBytes)
	}
	r.trace.LazyPrintf("%s", msg)

	if r.err != nil {
		r.trace.SetError()
		r.trace.LazyPrintf("%s", util.ErrorWithCauseChain(r.err))
	}
	r.trace.Finish()
}

func calculateDurationMillis(startTime time.Time) int64 {
	return time.Now().Sub(startTime).Nanoseconds() / time.Millisecond.Nanoseconds()
}
