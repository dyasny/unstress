// stress — cross-platform CPU and memory stress generator.
//
// Build:
//
//	Linux:   go build -o stress .
//	Windows: GOOS=windows GOARCH=amd64 go build -o stress.exe .
//
// Usage:
//
//	# 4 cores at 75% load, 50% RAM, run for 2 minutes
//	./stress --cpu-cores 4 --cpu-percent 75 --mem-percent 50 --duration 2m
//
//	# All cores at 100%, no memory pressure, run until Ctrl-C / SIGTERM
//	./stress --cpu-cores 0 --cpu-percent 100
//
//	# Memory only (30% RAM) for 30 seconds
//	./stress --cpu-cores 0 --mem-percent 30 --duration 30s
package main

import (
	"flag"
	"fmt"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"syscall"
	"time"
)

func main() {
	var (
		cpuCores   = flag.Int("cpu-cores", runtime.NumCPU(), "cores to stress; 0 = all cores")
		cpuPercent = flag.Int("cpu-percent", 100, "target CPU load per core, 0-100")
		memPercent = flag.Int("mem-percent", 0, "percentage of total RAM to hold, 0-100; 0 = disabled")
		duration   = flag.Duration("duration", 0, "how long to run, e.g. 30s, 5m, 1h; 0 = until signal")
		verbose    = flag.Bool("verbose", false, "print progress updates every second")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "Usage: stress [options]\n\nOptions:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	// ── Validation ────────────────────────────────────────────────────────
	if *cpuCores < 0 {
		fatalf("--cpu-cores must be >= 0 (0 means all cores)")
	}
	if *cpuPercent < 0 || *cpuPercent > 100 {
		fatalf("--cpu-percent must be between 0 and 100")
	}
	if *memPercent < 0 || *memPercent > 100 {
		fatalf("--mem-percent must be between 0 and 100")
	}
	if *cpuCores == 0 {
		*cpuCores = runtime.NumCPU()
	}

	// ── Print plan ────────────────────────────────────────────────────────
	totalRAM, err := totalRAMBytes()
	if err != nil {
		fatalf("could not query total RAM: %v", err)
	}
	targetMem := uint64(float64(totalRAM) * float64(*memPercent) / 100.0)

	fmt.Printf("stress: starting\n")
	fmt.Printf("  platform:     %s/%s\n", runtime.GOOS, runtime.GOARCH)
	fmt.Printf("  logical CPUs: %d\n", runtime.NumCPU())
	fmt.Printf("  total RAM:    %s\n", formatBytes(totalRAM))
	fmt.Printf("  cpu-cores:    %d\n", *cpuCores)
	fmt.Printf("  cpu-percent:  %d%%\n", *cpuPercent)
	fmt.Printf("  mem-percent:  %d%% (%s)\n", *memPercent, formatBytes(targetMem))
	if *duration > 0 {
		fmt.Printf("  duration:     %s\n", *duration)
	} else {
		fmt.Printf("  duration:     until SIGINT / SIGTERM\n")
	}
	fmt.Println()

	stop := make(chan struct{})
	var wg sync.WaitGroup

	// ── Memory stressor ───────────────────────────────────────────────────
	if *memPercent > 0 {
		if err := startMemoryStressor(targetMem, stop, &wg); err != nil {
			fatalf("memory stressor: %v", err)
		}
	}

	// ── CPU stressors ─────────────────────────────────────────────────────
	if *cpuCores > 0 && *cpuPercent > 0 {
		startCPUStressors(*cpuCores, *cpuPercent, stop, &wg)
	}

	// ── Optional progress ticker ──────────────────────────────────────────
	if *verbose {
		wg.Add(1)
		go func() {
			defer wg.Done()
			tick := time.NewTicker(time.Second)
			defer tick.Stop()
			start := time.Now()
			for {
				select {
				case <-stop:
					return
				case t := <-tick.C:
					fmt.Printf("  running %.0fs\n", t.Sub(start).Seconds())
				}
			}
		}()
	}

	// ── Wait for duration or signal ───────────────────────────────────────
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, os.Interrupt, syscall.SIGTERM)

	if *duration > 0 {
		select {
		case <-time.After(*duration):
			fmt.Println("\nstress: duration elapsed, shutting down...")
		case sig := <-sigCh:
			fmt.Printf("\nstress: received %s, shutting down...\n", sig)
		}
	} else {
		sig := <-sigCh
		fmt.Printf("\nstress: received %s, shutting down...\n", sig)
	}

	close(stop)
	wg.Wait()
	fmt.Println("stress: done")
}

// ── Utilities ─────────────────────────────────────────────────────────────────

func fatalf(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "stress: error: "+format+"\n", args...)
	os.Exit(1)
}

func formatBytes(b uint64) string {
	switch {
	case b >= 1<<30:
		return fmt.Sprintf("%.1f GiB", float64(b)/float64(1<<30))
	case b >= 1<<20:
		return fmt.Sprintf("%.1f MiB", float64(b)/float64(1<<20))
	case b >= 1<<10:
		return fmt.Sprintf("%.1f KiB", float64(b)/float64(1<<10))
	default:
		return fmt.Sprintf("%d B", b)
	}
}
