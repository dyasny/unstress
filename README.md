# UnStress

Simple cross platform stress generator for CPU and RAM - general-purpose, percentage-accurate stress tool

## Memory mechanism 

Allocate a `[]byte` of the target size, then touch every 4 KiB page to force physical backing (not just virtual address reservation). A background goroutine re-dirties pages every 500ms to prevent the OS from reclaiming them under pressure.

## CPU mechanism

Each goroutine repeats a burn / sleep cycle within a 10ms window. At 75% load, it burns for 7.5ms then sleeps for 2.5ms, continuously. From the OS scheduler's perspective, that core is running at 75% utilization.

The "work" being done is entirely calling time.Now() in a tight loop — which is just reading the system clock over and over. It's not mathematically meaningful work; it's purely a mechanism to consume cycles for a precise wall-clock duration without the compiler optimizing the loop away (an empty for {} would just be a branch to itself, which some compilers can fold, and time.Now() has a side effect so it can't be elided).

What this means in practice:

- It stresses the execution units and clock circuitry, but not any specific functional unit (no heavy FPU, no cache pressure, no branch misprediction storms)
- It's very accurate for load percentage because it's time-controlled, not work-controlled
- It does not stress the memory subsystem (that's the separate mem stressor), the FPU, or SIMD units
- The 10ms window is coarse enough that the OS scheduler sees smooth load, but fine enough that it responds quickly to changes

### NOTE

In Windows, `time.Sleep` has ~15ms default resolution, so very low CPU percentages (< 10%) will be less precise.

## Build

### Linux

```sh
go build -o stress .
```

### Windows (cross-compile from Linux)

```sh
GOOS=windows GOARCH=amd64 go build -o stress.exe .
```

### ARM64 Linux (e.g. Graviton, Apple Silicon)

```sh
GOARCH=arm64 go build -o stress .
```

## Usage
￼
```sh
# 4 cores at 75% load, hold 50% of RAM, for 2 minutes
./stress --cpu-cores 4 --cpu-percent 75 --mem-percent 50 --duration 2m

# All cores at 100%, no memory, until Ctrl-C/SIGTERM
./stress --cpu-cores 0 --cpu-percent 100

# Memory-only pressure (30% RAM), with progress output
./stress --cpu-cores 0 --mem-percent 30 --duration 1m --verbose
```

### Ansible task
￼
```yaml
- name: Run stress test for 5 minutes
  command: >
    /opt/stress
    --cpu-cores 4
    --cpu-percent 80
    --mem-percent 40
    --duration 5m
  async: 360      # slightly longer than duration
  poll: 10
```
