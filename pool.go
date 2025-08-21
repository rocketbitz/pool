package pool

import (
	"context"
	"sync"
	"sync/atomic"
)

// Pool defines a worker pool
type Pool struct {
	sem  chan struct{}
	job  func(ctx context.Context, input any)
	done chan struct{}
	once sync.Once
	wg   sync.WaitGroup

	// metrics
	started uint64
	run     uint64

	// callbacks
	mu        sync.RWMutex // protects callbacks
	callbacks []Callback
}

// New creates a new worker pool with a goroutine limit
// and a job function to execute on the incoming data
func New(routines int, job func(ctx context.Context, input any), pcbs ...Callback) *Pool {
	if routines <= 0 {
		routines = 1
	}

	sem := make(chan struct{}, routines)

	for i := 0; i < routines; i++ {
		sem <- struct{}{} // seed tokens
	}

	return &Pool{
		sem:       sem,
		job:       job,
		done:      make(chan struct{}),
		callbacks: append([]Callback(nil), pcbs...),
	}
}

// Work starts the pool working on a data input channel until the channel closes
// or the context is cancelled. All in-flight jobs are waited on before returning.
func (p *Pool) Work(ctx context.Context, c <-chan any) {
loop:
	for {
		select {
		case <-ctx.Done():
			break loop
		case v, ok := <-c:
			if !ok {
				break loop // channel closed
			}

			select {
			case <-ctx.Done():
				break loop
			case <-p.sem:
			}

			p.wg.Add(1)
			atomic.AddUint64(&p.started, 1)
			p.runCallbacks(JobStart)

			go func(input any) {
				defer func() {
					p.runCallbacks(JobEnd)
					p.wg.Done()
					p.sem <- struct{}{}
					atomic.AddUint64(&p.run, 1)
				}()
				p.job(ctx, input)
			}(v)
		}
	}

	p.wg.Wait()
	p.once.Do(func() { close(p.done) })
}

// Wait blocks until Work() has drained and all jobs have completed.
func (p *Pool) Wait() { <-p.done }

// Started returns the number of jobs that have begun.
func (p *Pool) Started() uint64 { return atomic.LoadUint64(&p.started) }

// Run returns the number of jobs that have finished.
func (p *Pool) Run() uint64 { return atomic.LoadUint64(&p.run) }

// JobEvent defines the kind of event upon which the PoolCallback is executed
type JobEvent int

const (
	// JobStart callbacks run just before the job is executed
	JobStart JobEvent = iota
	// JobEnd callbacks run just after the job has executed
	JobEnd
)

// Callback defines a function that is meant to be run each time the specified JobEvent occurs
type Callback struct {
	Func  func()
	Event JobEvent
}

// RegisterCallback registers a callback to be triggered by the pool.
// Safe to call while the pool is running.
func (p *Pool) RegisterCallback(pcb Callback) {
	p.mu.Lock()
	defer p.mu.Unlock()

	newCbs := make([]Callback, len(p.callbacks)+1)
	copy(newCbs, p.callbacks)
	newCbs[len(newCbs)-1] = pcb
	p.callbacks = newCbs
}

func (p *Pool) runCallbacks(evt JobEvent) {
	p.mu.RLock()
	cbs := p.callbacks
	p.mu.RUnlock()

	for _, pcb := range cbs {
		if pcb.Event == evt {
			pcb.Func()
		}
	}
}
