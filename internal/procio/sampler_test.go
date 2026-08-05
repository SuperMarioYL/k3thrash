package procio

import (
	"testing"
	"time"
)

func TestDefaultOptions(t *testing.T) {
	o := DefaultOptions()
	if o.Interval != 100*time.Millisecond {
		t.Errorf("Interval = %v, want 100ms (10Hz)", o.Interval)
	}
}

func TestNewOnNonLinuxReturnsErrLinuxOnly(t *testing.T) {
	// On Linux the real reader runs (covered by sampler_linux_test.go). On
	// non-Linux dev hosts New must return ErrLinuxOnly so the rest of the
	// toolchain still type-checks.
	s, err := New(1, DefaultOptions())
	if err == nil && s != nil {
		// Linux path: real sampler returned — nothing to assert here.
		return
	}
	if err != ErrLinuxOnly {
		t.Errorf("New on non-Linux: err = %v, want ErrLinuxOnly", err)
	}
}
