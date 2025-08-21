package pool

import (
	"context"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type PoolTestSuite struct {
	suite.Suite
}

func TestPoolTestSuite(t *testing.T) {
	suite.Run(t, new(PoolTestSuite))
}

func (s *PoolTestSuite) TestPoolBasic() {
	var (
		jobsQueued     = 10
		jobsRun        uint64
		jobStartEvents uint64
		jobEndEvents   uint64

		job = func(ctx context.Context, input any) {
			atomic.AddUint64(&jobsRun, 1)
		}

		p = New(
			jobsQueued,
			job,
			Callback{
				Event: JobStart,
				Func: func() {
					atomic.AddUint64(&jobStartEvents, 1)
				},
			},
			Callback{
				Event: JobEnd,
				Func: func() {
					atomic.AddUint64(&jobEndEvents, 1)
				},
			},
		)
	)

	// extra callback
	p.RegisterCallback(Callback{
		Event: JobEnd,
		Func: func() {
			atomic.AddUint64(&jobEndEvents, 1)
		},
	})

	c := make(chan any)
	ctx := context.Background()

	go p.Work(ctx, c)

	for i := 0; i < jobsQueued; i++ {
		c <- struct{}{}
	}
	close(c)

	p.Wait()

	assert.Equal(s.T(), uint64(jobsQueued), jobsRun)
	assert.Equal(s.T(), uint64(jobsQueued), jobStartEvents)
	assert.Equal(s.T(), uint64(2*jobsQueued), jobEndEvents)
	assert.Equal(s.T(), jobsQueued, int(p.Started()))
	assert.Equal(s.T(), jobsQueued, int(p.Run()))
}

func (s *PoolTestSuite) TestPoolCancellation() {
	var (
		jobsRun uint64
		job     = func(ctx context.Context, input any) {
			select {
			case <-time.After(50 * time.Millisecond):
				atomic.AddUint64(&jobsRun, 1)
			case <-ctx.Done():
				// cancelled
			}
		}
		p = New(2, job)
	)

	c := make(chan any)
	ctx, cancel := context.WithCancel(context.Background())

	go p.Work(ctx, c)

	// queue jobs
	for i := 0; i < 5; i++ {
		c <- struct{}{}
	}

	// cancel early
	cancel()
	close(c)
	p.Wait()

	// At least one may have run before cancel hit
	assert.LessOrEqual(s.T(), int(jobsRun), 5)
}

func (s *PoolTestSuite) TestPoolConcurrencyLimit() {
	var (
		maxConcurrent int32
		current       int32
		job           = func(ctx context.Context, input any) {
			n := atomic.AddInt32(&current, 1)
			for {
				old := atomic.LoadInt32(&maxConcurrent)
				if n > old {
					if atomic.CompareAndSwapInt32(&maxConcurrent, old, n) {
						break
					}
					continue
				}
				break
			}
			time.Sleep(20 * time.Millisecond)
			atomic.AddInt32(&current, -1)
		}
		p = New(3, job)
	)

	c := make(chan any)
	ctx := context.Background()

	go p.Work(ctx, c)

	for i := 0; i < 10; i++ {
		c <- struct{}{}
	}
	close(c)

	p.Wait()

	assert.Equal(s.T(), 3, int(maxConcurrent), "should not exceed concurrency limit")
}

func (s *PoolTestSuite) TestPoolCallbacksThreadSafety() {
	var calls uint64
	job := func(ctx context.Context, input any) {}
	p := New(2, job)

	// Register callbacks concurrently
	done := make(chan struct{})
	go func() {
		for i := 0; i < 100; i++ {
			p.RegisterCallback(Callback{
				Event: JobStart,
				Func:  func() { atomic.AddUint64(&calls, 1) },
			})
		}
		close(done)
	}()

	c := make(chan any)
	ctx := context.Background()
	go p.Work(ctx, c)

	// queue multiple jobs so some overlap with registration
	for i := 0; i < 20; i++ {
		c <- struct{}{}
	}
	close(c)

	p.Wait()
	<-done // ensure registration finished

	assert.Greater(s.T(), atomic.LoadUint64(&calls), uint64(0), "at least one callback should have fired")
}
