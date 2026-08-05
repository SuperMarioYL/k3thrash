// Package procio samples /proc/<pid>/io.read_bytes at a fixed cadence (10 Hz
// by default) to measure the NVMe read-rate the attached llama.cpp K3 process
// is generating. The sampler emits cumulative read_bytes counters; the thrash
// engine takes deltas to get bytes-per-token.
//
// The real /proc reader is Linux-only (build tag) because /proc/<pid>/io is a
// Linux procfs entry. Non-Linux hosts get an explicit ErrLinuxOnly stub so
// `go build ./...` and `go test ./...` still pass on this macOS dev host — the
// cross-platform logic (topo, thrash, trace, render) is unit-tested directly.
package procio

import "time"

// IOSample is one instantaneous reading of /proc/<pid>/io.read_bytes.
type IOSample struct {
	T         time.Time
	ReadBytes uint64
}

// Sampler reads cumulative read_bytes on demand.
type Sampler interface {
	Read() (IOSample, error)
	Close() error
}

// Options tunes the sampler. Interval is the suggested tick cadence used by the
// attach command (default 100 ms = 10 Hz per mvp_plan §2).
type Options struct {
	Interval time.Duration
}

// DefaultOptions returns the 10 Hz baseline the mvp plan specifies.
func DefaultOptions() Options {
	return Options{Interval: 100 * time.Millisecond}
}

// now is a seam for tests; the linux reader stamps each IOSample with it.
func now() time.Time { return time.Now() }

// New constructs a Sampler for the given pid. On Linux it opens
// /proc/<pid>/io; elsewhere it returns ErrLinuxOnly so callers can surface a
// clear message rather than a mysterious nil-pointer.
func New(pid int, _ Options) (Sampler, error) {
	return newProcSampler(pid)
}

// ErrLinuxOnly is returned by New on non-Linux hosts: the /proc/<pid>/io
// sampler only works where procfs exists (Linux). The attach command surfaces
// this verbatim so a macOS user running `go test` gets a clear message.
var ErrLinuxOnly = errLinuxOnly{}

type errLinuxOnly struct{}

func (errLinuxOnly) Error() string {
	return "k3thrash: /proc/<pid>/io sampler is Linux-only (procfs required); build with GOOS=linux or run on a Linux K3 rig"
}
