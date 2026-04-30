// Package task is a tiny progress-aware concurrency primitive that mirrors
// the @xmcl/task API used by the Electron launcher. A Task represents one
// install step (e.g. "download library X") that can be composed into a
// hierarchy and run concurrently with bounded parallelism.
package task

import (
	"context"
	"sync"
	"sync/atomic"

	"golang.org/x/sync/errgroup"
)

// Func is the unit of work. It receives the current task handle so it can
// update progress as it runs.
type Func func(ctx context.Context, t *Task) error

// Task is a single named step. It's safe to call Add/Set from any goroutine.
type Task struct {
	Name     string
	progress atomic.Int64
	total    atomic.Int64

	parent *Task
	rep    Reporter

	mu       sync.Mutex
	children []*Task
}

// Reporter is invoked on every progress update.
type Reporter interface {
	OnUpdate(t *Task)
	OnSucceed(t *Task)
	OnFailed(t *Task, err error)
}

// New constructs a top-level task with the given progress reporter.
func New(name string, rep Reporter) *Task {
	if rep == nil {
		rep = nopReporter{}
	}
	return &Task{Name: name, rep: rep}
}

// Sub creates a child task. The child's totals roll up into the parent.
func (t *Task) Sub(name string) *Task {
	c := &Task{Name: name, parent: t, rep: t.rep}
	t.mu.Lock()
	t.children = append(t.children, c)
	t.mu.Unlock()
	return c
}

// SetTotal records the total amount of work for this task.
func (t *Task) SetTotal(n int64) {
	delta := n - t.total.Load()
	t.total.Add(delta)
	if t.parent != nil {
		t.parent.total.Add(delta)
	}
	t.rep.OnUpdate(t.root())
}

// Add increments progress by n.
func (t *Task) Add(n int64) {
	t.progress.Add(n)
	if t.parent != nil {
		t.parent.progress.Add(n)
	}
	t.rep.OnUpdate(t.root())
}

// Progress returns the current cumulative progress.
func (t *Task) Progress() int64 { return t.progress.Load() }

// Total returns the cumulative total set so far.
func (t *Task) Total() int64 { return t.total.Load() }

// Run executes fn synchronously and reports success/failure.
func (t *Task) Run(ctx context.Context, fn Func) error {
	if err := fn(ctx, t); err != nil {
		t.rep.OnFailed(t, err)
		return err
	}
	t.rep.OnSucceed(t)
	return nil
}

// Parallel runs the supplied functions as child tasks of t with bounded
// parallelism (limit = max concurrent goroutines, 0 means unlimited).
func (t *Task) Parallel(ctx context.Context, limit int, fns []Func) error {
	g, gctx := errgroup.WithContext(ctx)
	if limit > 0 {
		g.SetLimit(limit)
	}
	for i, fn := range fns {
		fn := fn
		i := i
		g.Go(func() error {
			child := t.Sub(t.Name + "#" + itoa(i))
			return child.Run(gctx, fn)
		})
	}
	return g.Wait()
}

func (t *Task) root() *Task {
	cur := t
	for cur.parent != nil {
		cur = cur.parent
	}
	return cur
}

type nopReporter struct{}

func (nopReporter) OnUpdate(*Task)         {}
func (nopReporter) OnSucceed(*Task)        {}
func (nopReporter) OnFailed(*Task, error)  {}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := i < 0
	if neg {
		i = -i
	}
	var buf [20]byte
	pos := len(buf)
	for i > 0 {
		pos--
		buf[pos] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		pos--
		buf[pos] = '-'
	}
	return string(buf[pos:])
}
