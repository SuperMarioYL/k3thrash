package trace

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/SuperMarioYL/k3thrash/internal/topo"
)

func TestAppendAndDelta(t *testing.T) {
	tr := New("kimi-k3", 4242, topo.KimiK3())
	now := time.Now().UTC()

	// First sample: no previous → BytesPerToken stays 0.
	rate0 := tr.Append(Sample{T: now, ReadBytes: 0, TokensSoFar: 0})
	if rate0 != 0 {
		t.Errorf("first Append rate = %g, want 0", rate0)
	}

	// Second sample: +1 token, +1.6 GiB read → ~1.6 GiB/token.
	rate1 := tr.Append(Sample{T: now.Add(100 * time.Millisecond), ReadBytes: 1_610_612_736, TokensSoFar: 1})
	if rate1 <= 0 {
		t.Errorf("second Append rate = %g, want > 0", rate1)
	}
	// expect ≈ 1.6 GiB
	if rate1 < 1_500_000_000 || rate1 > 1_700_000_000 {
		t.Errorf("second Append rate = %g, want ~1.6 GiB", rate1)
	}

	if len(tr.Samples) != 2 {
		t.Errorf("len(samples) = %d, want 2", len(tr.Samples))
	}
}

func TestAppendNoTokenAdvance(t *testing.T) {
	tr := New("kimi-k3", 1, topo.KimiK3())
	now := time.Now().UTC()
	tr.Append(Sample{T: now, ReadBytes: 100, TokensSoFar: 5})
	// Δtokens == 0 → rate must be 0 (no divide-by-zero).
	rate := tr.Append(Sample{T: now.Add(100 * time.Millisecond), ReadBytes: 200, TokensSoFar: 5})
	if rate != 0 {
		t.Errorf("rate with Δtokens=0 = %g, want 0", rate)
	}
}

func TestRateSeries(t *testing.T) {
	tr := New("kimi-k3", 1, topo.KimiK3())
	now := time.Now().UTC()
	tr.Append(Sample{T: now, ReadBytes: 0, TokensSoFar: 0})         // no delta
	tr.Append(Sample{T: now.Add(time.Second), ReadBytes: 100, TokensSoFar: 1})   // 100 B/tok
	tr.Append(Sample{T: now.Add(2 * time.Second), ReadBytes: 350, TokensSoFar: 2}) // 250 B/tok
	got := tr.RateSeries()
	if len(got) != 2 {
		t.Fatalf("len(RateSeries) = %d, want 2", len(got))
	}
	if got[0] != 100 || got[1] != 250 {
		t.Errorf("RateSeries = %v, want [100 250]", got)
	}
}

func TestFinalizeSetsVerdict(t *testing.T) {
	tr := New("kimi-k3", 1, topo.KimiK3())
	now := time.Now().UTC()
	baseline := 25_000_000_000.0 // pathological ~25 GiB/token
	// 3 samples each ~3× baseline → pathological
	tr.Append(Sample{T: now, ReadBytes: 0, TokensSoFar: 0})
	tr.Append(Sample{T: now.Add(time.Second), ReadBytes: uint64(3 * baseline), TokensSoFar: 1})
	tr.Append(Sample{T: now.Add(2 * time.Second), ReadBytes: uint64(6 * baseline), TokensSoFar: 2})

	v := tr.Finalize()
	if v.Classification == "" {
		t.Error("Finalize verdict classification empty")
	}
	if tr.FinalVerdict == nil {
		t.Error("FinalVerdict not set on trace")
	}
	if !tr.EndedAt.IsZero() && tr.EndedAt.Before(tr.StartedAt) {
		t.Error("EndedAt before StartedAt")
	}
}

func TestWriteReadRoundTrip(t *testing.T) {
	tr := New("kimi-k3", 7777, topo.KimiK3())
	now := time.Now().UTC().Truncate(time.Second)
	tr.StartedAt = now
	tr.Append(Sample{T: now, ReadBytes: 0, TokensSoFar: 0})
	tr.Append(Sample{T: now.Add(time.Second), ReadBytes: 1_610_612_736, TokensSoFar: 1})
	tr.Finalize()

	dir := t.TempDir()
	path := filepath.Join(dir, "trace.json")
	if err := tr.WriteFile(path); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}

	loaded, err := ReadFile(path)
	if err != nil {
		t.Fatalf("ReadFile: %v", err)
	}
	if loaded.Model != tr.Model {
		t.Errorf("Model = %q, want %q", loaded.Model, tr.Model)
	}
	if loaded.PID != tr.PID {
		t.Errorf("PID = %d, want %d", loaded.PID, tr.PID)
	}
	if len(loaded.Samples) != 2 {
		t.Errorf("loaded samples = %d, want 2", len(loaded.Samples))
	}
	if loaded.FinalVerdict == nil {
		t.Error("loaded FinalVerdict nil")
	}
}

func TestWriteFileAtomicTmp(t *testing.T) {
	tr := New("kimi-k3", 1, topo.KimiK3())
	dir := t.TempDir()
	path := filepath.Join(dir, "out.json")
	if err := tr.WriteFile(path); err != nil {
		t.Fatalf("WriteFile: %v", err)
	}
	// .tmp sidecar must be gone after rename.
	if _, err := os.Stat(path + ".tmp"); !os.IsNotExist(err) {
		t.Errorf("expected .tmp to be removed, stat err = %v", err)
	}
	// valid JSON
	b, _ := os.ReadFile(path)
	var raw map[string]any
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Errorf("output not valid JSON: %v", err)
	}
}
