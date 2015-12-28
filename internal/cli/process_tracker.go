package cli

import (
	"fmt"
	"sync"

	"golang.org/x/net/context"
)

const processTrackerEventFamily = "cli.ProcessTracker"

// ProcessTracker manages multiple goroutines that are deduped by a key and are associated
// with a stop channel.
type ProcessTracker struct {
	// Protects access to processesByKey.
	lock           sync.Mutex
	processesByKey map[string]*ProcessInfo
	processWaiter  sync.WaitGroup

	// Used as base contexts for individual processes.
	baseContext context.Context
	cancelFunc  context.CancelFunc

	eventLog *EventLog
}

type ProcessInfo struct {
	context    context.Context
	cancelFunc context.CancelFunc

	eventLog *EventLog
}

type Process func(key string, ctx context.Context)

func NewProcessTracker() *ProcessTracker {
	context, cancelFunc := context.WithCancel(context.Background())
	return &ProcessTracker{
		processesByKey: make(map[string]*ProcessInfo),
		baseContext:    context,
		cancelFunc:     cancelFunc,
		eventLog:       NewEventLog(processTrackerEventFamily, ""),
	}
}

func (t *ProcessTracker) Go(key string, proc Process) (procInfo *ProcessInfo, err error) {
	if isContextAlreadyDone(t.baseContext) {
		return nil, fmt.Errorf("process tracker has been shutdown")
	}

	t.lock.Lock()
	defer t.lock.Unlock()

	if _, ok := t.processesByKey[key]; ok {
		return nil, fmt.Errorf("process already running for key %s", key)
	}

	context, cancelFunc := context.WithCancel(t.baseContext)
	procInfo = &ProcessInfo{
		context:    context,
		cancelFunc: cancelFunc,
		eventLog:   NewEventLog(processTrackerEventFamily, "key:"+key),
	}
	t.processesByKey[key] = procInfo
	t.processWaiter.Add(1)

	go func() {
		defer t.sweep(key)
		procInfo.eventLog.Debugf("started process for key %s", key)
		proc(key, context)
		procInfo.eventLog.Debugf("process exited normally for key %s", key)

		// If process exited on its own, without anyone calling shutdown, we need to call this so
		// sweep doesn't panic.
		cancelFunc()
	}()

	return
}

func (t *ProcessTracker) Shutdown() {
	t.eventLog.Debugf("shutting down...")

	t.cancelFunc()
	t.processWaiter.Wait()

	t.eventLog.Debugf("shutdown.")
	t.eventLog.Finish()
}

// sweep is called from the goroutine that the process executes in.
func (t *ProcessTracker) sweep(key string) {
	t.lock.Lock()
	defer t.lock.Unlock()

	if procInfo, ok := t.processesByKey[key]; ok {
		if !isContextAlreadyDone(procInfo.context) {
			panic("trying to sweep process before it's done for key " + key)
		}

		procInfo.eventLog.Finish()
		delete(t.processesByKey, key)
		t.processWaiter.Done()

		t.eventLog.Debugf("swept process for key %s", key)
	}
}

func isContextAlreadyDone(ctx context.Context) bool {
	select {
	case <-ctx.Done():
		return true
	default:
		return false
	}
}
