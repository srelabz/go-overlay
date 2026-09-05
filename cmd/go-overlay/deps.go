package main

import "sync"

type dependencyTracker struct {
	ready  map[string]chan struct{}
	failed map[string]chan struct{}
	closed map[string]bool
	mu     sync.Mutex
}

func newDependencyTracker() *dependencyTracker {
	return &dependencyTracker{
		ready:  make(map[string]chan struct{}),
		failed: make(map[string]chan struct{}),
		closed: make(map[string]bool),
	}
}

func (d *dependencyTracker) channel(store map[string]chan struct{}, name string) chan struct{} {
	ch, ok := store[name]
	if !ok {
		ch = make(chan struct{})
		store[name] = ch
	}
	return ch
}

func (d *dependencyTracker) mark(store map[string]chan struct{}, kind, name string) {
	d.mu.Lock()
	defer d.mu.Unlock()

	key := kind + ":" + name
	if d.closed[key] {
		return
	}
	d.closed[key] = true
	close(d.channel(store, name))
}

func (d *dependencyTracker) MarkReady(name string) {
	d.mark(d.ready, "ready", name)
}

func (d *dependencyTracker) MarkFailed(name string) {
	d.mark(d.failed, "failed", name)
}

func (d *dependencyTracker) Ready(name string) <-chan struct{} {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.channel(d.ready, name)
}

func (d *dependencyTracker) Failed(name string) <-chan struct{} {
	d.mu.Lock()
	defer d.mu.Unlock()
	return d.channel(d.failed, name)
}

func (d *dependencyTracker) IsReady(name string) bool {
	select {
	case <-d.Ready(name):
		return true
	default:
		return false
	}
}

func (d *dependencyTracker) IsFailed(name string) bool {
	select {
	case <-d.Failed(name):
		return true
	default:
		return false
	}
}
