//go:build linux

package main

import (
	"bufio"
	"fmt"
	"os"
	"strconv"
	"strings"
)

// totalRAMBytes returns the total physical RAM in bytes by reading
// /proc/meminfo. Uses MemTotal (installed RAM), not MemAvailable, so
// --mem-percent is a fraction of the machine's physical memory regardless
// of how much is currently in use.
func totalRAMBytes() (uint64, error) {
	f, err := os.Open("/proc/meminfo")
	if err != nil {
		return 0, fmt.Errorf("open /proc/meminfo: %w", err)
	}
	defer f.Close()

	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "MemTotal:") {
			continue
		}
		// Format: "MemTotal:       16291056 kB"
		fields := strings.Fields(line)
		if len(fields) < 3 || fields[2] != "kB" {
			return 0, fmt.Errorf("unexpected MemTotal format: %q", line)
		}
		kb, err := strconv.ParseUint(fields[1], 10, 64)
		if err != nil {
			return 0, fmt.Errorf("parse MemTotal value: %w", err)
		}
		return kb * 1024, nil
	}
	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("read /proc/meminfo: %w", err)
	}
	return 0, fmt.Errorf("MemTotal not found in /proc/meminfo")
}
