//go:build !linux

package procio

// newProcSampler returns the Linux-only error on non-Linux hosts. This stub
// exists only so the package (and everything that imports it: cmd/k3thrash,
// trace, render) still type-checks and tests cleanly on macOS dev hosts.
// The cross-platform verdict/trace/render logic is exercised by the package
// tests without ever calling New.
func newProcSampler(pid int) (Sampler, error) {
	_ = pid
	return nil, ErrLinuxOnly
}
