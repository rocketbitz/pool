# pool
a simple worker pool for gophers

# installation

as with all other go packages, you know what to do:
```
go get github.com/rocketbitz/pool
```

# usage

this worker pool package was designed to be straightforward, specifically because I found others to be unnecessarily complex. here's an example:
```go
package main

import (
	"fmt"

	"github.com/rocketbitz/pool"
)

func main() {
	numWorkers := 10
	jobToRun := func(input interface{}) {
		fmt.Println("let's do a job on this input: ", input)
	}

	jobStartCallback := pool.Callback{
		Event: pool.JobStart,
		Func: func() {
			fmt.Println("we're starting a job...")
		},
	}

	jobEndCallback := pool.Callback{
		Event: pool.JobEnd,
		Func: func() {
			fmt.Println("we've finished a job.")
		},
	}

	p := pool.New(
		numWorkers,
		jobToRun,
		jobStartCallback,
		jobEndCallback,
	)

	c := make(chan interface{})

	go p.Work(c)

	for i := 0; i < 100; i++ {
		c <- "hello, gophers"
	}

	close(c)
	p.Wait()
}
```
hopefully that example makes sense to you. note that the callback argument to `New()` is variadic, so register as many callbacks as your heart desires. oh, one more thing, if you forget a callback when you declare the pool, you can always register one later like so:
```go
p.RegisterCallback(
  Callback{
    Event: JobEnd,
    Func: func() { fmt.Println("i'm a forgetful gopher") },
  },
)
```

# contribute

pr's are welcome. if they're awesome, they'll get reviewed and merged. if they're not, they'll get reviewed and closed, hopefully with a kind comment as to the reason.

# license

[MIT](https://github.com/rocketbitz/pool/blob/master/LICENSE) ...move along, move along.
