package cli

import (
	"fmt"
	"sync"

	"golang.org/x/net/context"
)

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
}

type ProcessInfo struct {
	context    context.Context
	cancelFunc context.CancelFunc
}

type Process func(key string, ctx context.Context)

func NewProcessTracker() *ProcessTracker {
	context, cancelFunc := context.WithCancel(context.Background())
	return &ProcessTracker{
		processesByKey: make(map[string]*ProcessInfo),
		baseContext:    context,
		cancelFunc:     cancelFunc,
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
	}
	t.processesByKey[key] = procInfo
	t.processWaiter.Add(1)

	go func() {
		defer t.sweep(key)
		proc(key, context)
		// If process exited on its own, without anyone calling shutdown, we need to call this so
		// sweep doesn't panic.
		cancelFunc()
	}()

	return
}

func (t *ProcessTracker) Shutdown() {
	t.cancelFunc()
	t.processWaiter.Wait()
}

// sweep is called from the goroutine that the process executes in.
func (t *ProcessTracker) sweep(key string) {
	t.lock.Lock()
	defer t.lock.Unlock()

	if procInfo, ok := t.processesByKey[key]; ok {
		if !isContextAlreadyDone(procInfo.context) {
			panic("trying to sweep process before it's done for key " + key)
		}

		delete(t.processesByKey, key)
		t.processWaiter.Done()
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
