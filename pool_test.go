package pool

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/suite"
)

type PoolTestSuite struct {
	suite.Suite
}

func TestPoolTestSuite(t *testing.T) {
	suite.Run(t, new(PoolTestSuite))
}

func (s *PoolTestSuite) TestPool() {
	var (
		jobsQueued     = 10
		jobsRun        uint64
		jobStartEvents uint64
		jobEndEvents   uint64

		job = func(input interface{}) {
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

	p.RegisterCallback(Callback{
		Event: JobEnd,
		Func: func() {
			atomic.AddUint64(&jobEndEvents, 1)
		},
	})

	c := make(chan interface{})

	go p.Work(c)

	for i := 0; i < jobsQueued; i++ {
		c <- struct{}{}
	}

	close(c)

	p.Wait()

	assert.Equal(s.T(), uint64(jobsQueued), jobsRun)
	assert.Equal(s.T(), uint64(jobsQueued), jobStartEvents)
	assert.Equal(s.T(), uint64(2*jobsQueued), jobEndEvents)
}
