package main

import (
	"runtime"
	"sync"
	"time"
)

// windowSize is the duty-cycle period. Each CPU worker burns CPU for
// (percent/100 * windowSize) then sleeps the remainder, producing the
// target average load.
//
// Smaller windows = more accurate load at the cost of more scheduler
// overhead. 10 ms is a good balance; it's also the Linux HZ period.
//
// Note: on Windows, Sleep() has ~15 ms resolution by default (the OS
// timer resolution). For very low percentages (e.g. 5%) on Windows the
// effective load will be slightly coarser — use a larger window if that
// matters, or call timeBeginPeriod(1) before starting.
const windowSize = 10 * time.Millisecond

// startCPUStressors launches `cores` goroutines, each pinned to an OS
// thread, running a busy/sleep duty cycle to produce `percent`% CPU load.
func startCPUStressors(cores, percent int, stop <-chan struct{}, wg *sync.WaitGroup) {
	workTime := time.Duration(float64(windowSize) * float64(percent) / 100.0)
	idleTime := windowSize - workTime

	for i := 0; i < cores; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()

			// Lock this goroutine to its OS thread so it cannot be
			// migrated; this gives cleaner per-core utilisation numbers
			// in tools like htop / Task Manager.
			runtime.LockOSThread()
			defer runtime.UnlockOSThread()

			cpuWorker(workTime, idleTime, stop)
		}()
	}
}

// cpuWorker runs a tight busy loop for workTime, then sleeps for idleTime,
// repeating until stop is closed.
func cpuWorker(workTime, idleTime time.Duration, stop <-chan struct{}) {
	for {
		// Check for stop at the top of each cycle (low overhead).
		select {
		case <-stop:
			return
		default:
		}

		if workTime > 0 {
			// Spin-burn for workTime. Using time.Now() inside the hot
			// loop is intentional — it prevents the compiler from
			// optimising the loop away and itself consumes a small but
			// real number of cycles, which is fine for a stress tool.
			deadline := time.Now().Add(workTime)
			for time.Now().Before(deadline) {
				// Intentional busy loop — no body.
			}
		}

		if idleTime > 0 {
			time.Sleep(idleTime)
		}
	}
}
