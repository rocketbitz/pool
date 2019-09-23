package pool

import (
	"sync/atomic"
)

// Pool defines a worker pool
type Pool struct {
	workerQ      chan struct{}
	f            func(input interface{})
	callbacks    []Callback
	done         chan struct{}
	started, run uint64
}

// New creates a new worker pool with a goroutine limit
// and a job function to execute on the incoming data
func New(routines int, job func(input interface{}), pcbs ...Callback) *Pool {
	q := make(chan struct{}, routines)

	for i := 0; i < routines; i++ {
		q <- struct{}{}
	}

	return &Pool{
		workerQ:   q,
		f:         job,
		callbacks: append([]Callback{}, pcbs...),
		done:      make(chan struct{}),
	}
}

// Work is a blocking call that starts the
// pool working on a data input channel
func (p *Pool) Work(c <-chan interface{}) {
	var (
		v    interface{}
		more bool
	)

	defer func() { p.done <- struct{}{} }()

	for {
		v, more = <-c

		// the work channel was closed, let's exit
		if !more {
			return
		}

		atomic.AddUint64(&p.started, 1)

		<-p.workerQ

		// trigger the JobStart callbacks before
		// spawning the goroutine
		p.runCallbacks(JobStart)

		go func(input interface{}) {
			// run the job
			p.f(input)

			p.runCallbacks(JobEnd)
			p.workerQ <- struct{}{}

			atomic.AddUint64(&p.run, 1)
		}(v)
	}
}

// Wait waits until the pool is finished
func (p *Pool) Wait() {
	<-p.done
	for {
		if p.started == p.run {
			return
		}
	}
}

// JobEvent defines the kind of event upon which
// the PoolCallback is executed
type JobEvent int

const (
	// JobStart callbacks run just before the job
	// is executed
	JobStart JobEvent = iota

	// JobEnd callbacks run just after the job
	// has executed
	JobEnd
)

// Callback defines a function that is meant to
// be run each time the specified JobEvent occurs
type Callback struct {
	Func  func()
	Event JobEvent
}

// RegisterCallback registers a callback to be
// triggered by the pool
func (p *Pool) RegisterCallback(pcb Callback) {
	p.callbacks = append(p.callbacks, pcb)
}

func (p *Pool) runCallbacks(evt JobEvent) {
	for _, pcb := range p.callbacks {
		if pcb.Event == evt {
			pcb.Func()
		}
	}
}
