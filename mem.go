package main

import (
	"fmt"
	"sync"
	"time"
)

// pageSize matches the OS page size (4 KiB on x86/ARM).
// Writing one byte per page is sufficient to fault every page into
// physical RAM; writing to every byte would be slower with no benefit.
const pageSize = 4096

// reTouchInterval controls how often we re-dirty the held memory.
// Modern OSes can reclaim clean anonymous pages under pressure; keeping
// them dirty prevents that. 500 ms is conservative; increase to 2–5 s
// for lower overhead on very large allocations.
const reTouchInterval = 500 * time.Millisecond

// startMemoryStressor allocates `targetBytes` of memory, touches every
// page to guarantee physical allocation, then periodically re-dirties the
// pages so the OS cannot silently reclaim them.
func startMemoryStressor(targetBytes uint64, stop <-chan struct{}, wg *sync.WaitGroup) error {
	if targetBytes == 0 {
		return nil
	}

	// Allocate upfront. If this panics (OOM) we surface a clear error.
	buf, err := allocate(targetBytes)
	if err != nil {
		return fmt.Errorf("allocation of %s failed: %w", formatBytes(targetBytes), err)
	}

	fmt.Printf("stress: memory allocated (%s), touching pages...\n", formatBytes(uint64(len(buf))))

	// Touch every page to fault it into physical RAM.
	touchAll(buf)

	fmt.Printf("stress: memory fully committed\n")

	wg.Add(1)
	go func() {
		defer wg.Done()
		memWorker(buf, stop)
	}()

	return nil
}

// allocate creates a slice of exactly n bytes, recovering a panic (OOM)
// and returning it as an error.
func allocate(n uint64) (buf []byte, err error) {
	defer func() {
		if r := recover(); r != nil {
			err = fmt.Errorf("runtime panic: %v", r)
		}
	}()
	buf = make([]byte, n)
	return buf, nil
}

// touchAll writes one byte to every OS page in buf, forcing the kernel to
// back each page with a physical frame (page fault on first write).
func touchAll(buf []byte) {
	for i := 0; i < len(buf); i += pageSize {
		buf[i] = 0xAA
	}
}

// memWorker keeps the allocation live and periodically re-dirties pages
// so the OS cannot silently swap or reclaim them.
func memWorker(buf []byte, stop <-chan struct{}) {
	ticker := time.NewTicker(reTouchInterval)
	defer ticker.Stop()

	// Track position for a rolling re-touch: on each tick we dirty the
	// next 10% of the buffer (in page-aligned steps), cycling round.
	// This keeps the whole allocation warm without doing a full scan on
	// every tick.
	pos := 0
	chunkSize := len(buf) / 10
	if chunkSize < pageSize {
		chunkSize = pageSize
	}

	for {
		select {
		case <-stop:
			return
		case <-ticker.C:
			end := pos + chunkSize
			if end > len(buf) {
				end = len(buf)
			}
			for i := pos; i < end; i += pageSize {
				buf[i]++ // increment to ensure a dirty write, not a no-op store
			}
			pos = end % len(buf)
		}
	}
}
