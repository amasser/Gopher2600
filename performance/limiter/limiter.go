// Package limiter provides a rough and ready way of limiting events to a fixed
// rate.
//
// A new FpsLimiter can be creating with (error handling removed for clarity):
//
//	fps, _ := limiter.NewFPSLimiter(60)
//
// Operations can then be stalled with the Wait() function. For example:
//
//	for {
//		fps.Wait()
//		renderImage()
//	}
package limiter

import (
	"fmt"
	"time"
)

// this is a really rough attempt at frame rate limiting. probably only any
// good if base performance of the machine is well above the required rate.

// FpsLimiter will trigger every frames per second
type FpsLimiter struct {
	framesPerSecond int
	secondsPerFrame time.Duration

	tick chan bool
}

// NewFPSLimiter is the preferred method of initialisation for FpsLimiter type
func NewFPSLimiter(framesPerSecond int) (*FpsLimiter, error) {
	lim := &FpsLimiter{}
	lim.SetLimit(framesPerSecond)

	lim.tick = make(chan bool)

	// run ticker concurrently
	go func() {
		adjustedSecondPerFrame := lim.secondsPerFrame
		t := time.Now()
		for {
			lim.tick <- true
			time.Sleep(adjustedSecondPerFrame)
			nt := time.Now()
			adjustedSecondPerFrame -= nt.Sub(t) - lim.secondsPerFrame
			t = nt
		}
	}()

	return lim, nil
}

// SetLimit changes the limit at which the FpsLimiter waits
func (lim *FpsLimiter) SetLimit(framesPerSecond int) {
	lim.framesPerSecond = framesPerSecond
	lim.secondsPerFrame, _ = time.ParseDuration(fmt.Sprintf("%fs", float64(1.0)/float64(framesPerSecond)))
}

// Wait will block until trigger
func (lim *FpsLimiter) Wait() {
	<-lim.tick
}

// HasWaited will return true if time has already elapsed and false it it is
// still yet to happen
func (lim *FpsLimiter) HasWaited() bool {
	select {
	case <-lim.tick:
		return true
	default:
		// default case means that the channel receiving case doesn't block
		return false
	}
}
