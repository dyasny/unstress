//go:build windows

package main

import (
	"fmt"
	"syscall"
	"unsafe"
)

// MEMORYSTATUSEX mirrors the Win32 MEMORYSTATUSEX structure.
// https://learn.microsoft.com/en-us/windows/win32/api/sysinfoapi/ns-sysinfoapi-memorystatusex
type memoryStatusEx struct {
	dwLength                uint32
	dwMemoryLoad            uint32
	ullTotalPhys            uint64
	ullAvailPhys            uint64
	ullTotalPageFile        uint64
	ullAvailPageFile        uint64
	ullTotalVirtual         uint64
	ullAvailVirtual         uint64
	ullAvailExtendedVirtual uint64
}

// totalRAMBytes returns the total physical RAM in bytes by calling
// kernel32.GlobalMemoryStatusEx. Uses ullTotalPhys (installed RAM), not
// ullAvailPhys, so --mem-percent is a fraction of the machine's physical
// memory regardless of how much is currently in use.
func totalRAMBytes() (uint64, error) {
	kernel32 := syscall.NewLazyDLL("kernel32.dll")
	proc := kernel32.NewProc("GlobalMemoryStatusEx")

	var ms memoryStatusEx
	ms.dwLength = uint32(unsafe.Sizeof(ms))

	ret, _, err := proc.Call(uintptr(unsafe.Pointer(&ms)))
	if ret == 0 {
		// err is the last Win32 error from GetLastError()
		return 0, fmt.Errorf("GlobalMemoryStatusEx failed: %w", err)
	}
	return ms.ullTotalPhys, nil
}
