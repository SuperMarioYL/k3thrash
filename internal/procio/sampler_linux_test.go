//go:build linux

package procio

import (
	"testing"
)

func TestLinuxReaderBadPid(t *testing.T) {
	// pid 999999 almost certainly does not exist → stat should fail.
	_, err := newProcSampler(999999)
	if err == nil {
		t.Skip("pid 999999 exists on this host; skipping")
	}
}
