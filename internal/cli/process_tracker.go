package cli

import (
	"fmt"
	"sync"

	"github.com/zach-klippenstein/adbfs"
)

// ProcessTracker manages multiple goroutines that are deduped by a key and are associated
// with a stop channel.
type ProcessTracker struct {
	lock           sync.Mutex
	processesByKey map[string]*ProcessInfo
	processWaiter  sync.WaitGroup
	isShutdown     adbfs.AtomicBool
}

type ProcessInfo struct {
	stop       chan struct{}
	isShutdown adbfs.AtomicBool
}

func (i *ProcessInfo) shutdown() {
	if i.isShutdown.CompareAndSwap(false, true) {
		close(i.stop)
	}
}

type Process func(key string, stop <-chan struct{})

func NewProcessTracker() *ProcessTracker {
	return &ProcessTracker{
		processesByKey: make(map[string]*ProcessInfo),
	}
}

func (t *ProcessTracker) Go(key string, proc Process) (procInfo *ProcessInfo, err error) {
	t.lock.Lock()
	defer t.lock.Unlock()

	if t.isShutdown.Value() {
		return nil, fmt.Errorf("process tracker has been shutdown")
	}

	if _, ok := t.processesByKey[key]; ok {
		return nil, fmt.Errorf("process already running for key %s", key)
	}

	procInfo = &ProcessInfo{
		stop: make(chan struct{}),
	}
	t.processesByKey[key] = procInfo
	t.processWaiter.Add(1)

	go func() {
		defer t.sweep(key)
		proc(key, procInfo.stop)
		// If process exited on its own, without anyone calling shutdown, we need to set this so
		// sweep doesn't panic.
		procInfo.isShutdown.CompareAndSwap(false, true)
	}()

	return
}

func (t *ProcessTracker) Shutdown() {
	if t.isShutdown.CompareAndSwap(false, true) {
		t.shutdownAll()
		t.processWaiter.Wait()
	}
}

func (t *ProcessTracker) shutdownAll() {
	t.lock.Lock()
	defer t.lock.Unlock()

	for _, v := range t.processesByKey {
		v.shutdown()
	}
}

func (t *ProcessTracker) sweep(key string) {
	t.lock.Lock()
	defer t.lock.Unlock()

	if procInfo, ok := t.processesByKey[key]; ok {
		if !procInfo.isShutdown.Value() {
			panic("trying to sweep process before its done for key " + key)
		}

		delete(t.processesByKey, key)
		t.processWaiter.Done()
	}
}
