package main

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"testing"

	"github.com/SuperMarioYL/k3thrash/internal/topo"
	"github.com/SuperMarioYL/k3thrash/internal/trace"
)

func TestExampleTraceMatchesTopo(t *testing.T) {
	// The shipped example must carry the baseline the code actually uses:
	// v0.2.0 shipped a transcription error (25769797776 vs the real
	// 16 active × 1610612736 = 25769803776) in the fixture and every
	// rendered demo text.
	raw, err := os.ReadFile(filepath.Join("..", "..", "examples", "trace.example.json"))
	if err != nil {
		t.Fatalf("read example trace: %v", err)
	}
	var tr trace.Trace
	if err := json.Unmarshal(raw, &tr); err != nil {
		t.Fatalf("parse example trace: %v", err)
	}
	if tr.FinalVerdict == nil {
		t.Fatal("example trace has no final verdict")
	}
	want := topo.ExpectedMinReadPerToken(topo.KimiK3())
	if tr.FinalVerdict.ExpectedMinReadPerToken != want {
		t.Errorf("expected_min_read_per_token = %v, want %v (n_active × packed expert bytes)",
			tr.FinalVerdict.ExpectedMinReadPerToken, want)
	}
	if tr.FinalVerdict.NvmeBytesPerToken <= 0 {
		t.Fatalf("example nvme_bytes_per_token = %v, want a positive rate", tr.FinalVerdict.NvmeBytesPerToken)
	}
	wantReuse := want / tr.FinalVerdict.NvmeBytesPerToken
	if math.Abs(tr.FinalVerdict.ReuseRatio-wantReuse) > 1e-9 {
		t.Errorf("reuse_ratio = %v, want %v (expected/actual)", tr.FinalVerdict.ReuseRatio, wantReuse)
	}
}
