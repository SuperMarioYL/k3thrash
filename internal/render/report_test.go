package render

import (
	"strings"
	"testing"

	"github.com/SuperMarioYL/k3thrash/internal/thrash"
	"github.com/SuperMarioYL/k3thrash/internal/topo"
	"github.com/SuperMarioYL/k3thrash/internal/trace"
)

func TestSparkline(t *testing.T) {
	cases := []struct {
		name   string
		values []float64
		want   string
	}{
		{"empty", []float64{}, ""},
		{"single", []float64{5}, "▅"},
		{"two_steps", []float64{0, 7}, "▁█"},
		{"ascending_9", []float64{0, 1, 2, 3, 4, 5, 6, 7, 8}, "▁▁▂▃▄▅▆▇█"},
		{"all_equal", []float64{3, 3, 3, 3}, "▅▅▅▅"},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Sparkline(c.values)
			if got != c.want {
				t.Errorf("Sparkline(%v) = %q, want %q", c.values, got, c.want)
			}
		})
	}
}

func TestSparklineLength(t *testing.T) {
	vals := []float64{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}
	got := Sparkline(vals)
	// one rune per value
	if rc := len([]rune(got)); rc != len(vals) {
		t.Errorf("rune count = %d, want %d", rc, len(vals))
	}
}

func TestReportContainsVerdictAndSpark(t *testing.T) {
	tr := trace.New("kimi-k3", 4242, topo.KimiK3())
	now := tr.StartedAt
	baseline := 25_000_000_000.0
	tr.Append(trace.Sample{T: now, ReadBytes: 0, TokensSoFar: 0})
	tr.Append(trace.Sample{T: now.Add(1e9), ReadBytes: uint64(3 * baseline), TokensSoFar: 1})
	tr.Append(trace.Sample{T: now.Add(2e9), ReadBytes: uint64(6 * baseline), TokensSoFar: 2})
	tr.Append(trace.Sample{T: now.Add(3e9), ReadBytes: uint64(9 * baseline), TokensSoFar: 3})
	tr.Finalize()

	out := Report(tr)

	mustContain := []string{
		"k3thrash report",
		"verdict",
		"re-read rate",
		"pathological thrash", // 3× baseline → pathological
		"sparkline",
		"share",
	}
	for _, s := range mustContain {
		if !strings.Contains(out, s) {
			t.Errorf("Report() missing %q\n--- output ---\n%s", s, out)
		}
	}
	// sparkline runes present
	if !strings.ContainsAny(out, "▁▂▃▄▅▆▇█") {
		t.Errorf("Report() output has no sparkline runes\n%s", out)
	}
}

func TestReportWithoutVerdict(t *testing.T) {
	// Trace that was never Finalized — report must still render, no panic.
	tr := trace.New("kimi-k3", 1, topo.KimiK3())
	out := Report(tr)
	if !strings.Contains(out, "not finalized") {
		t.Errorf("expected 'not finalized' marker for unfinished trace, got:\n%s", out)
	}
}

func TestVerdictSummaryIntegration(t *testing.T) {
	// Sanity: thrash.Verdict.Summary via a finalized trace renders cleanly.
	tr := trace.New("kimi-k3", 1, topo.KimiK3())
	now := tr.StartedAt
	tr.Append(trace.Sample{T: now, ReadBytes: 0, TokensSoFar: 0})
	tr.Append(trace.Sample{T: now.Add(1e9), ReadBytes: 1_610_612_736, TokensSoFar: 1})
	v := tr.Finalize()
	if v.Classification != thrash.HealthyWarmup && v.Classification != thrash.PartialWarm && v.Classification != thrash.PathologicalThrash {
		t.Errorf("final verdict classification = %q, want one of the known buckets", v.Classification)
	}
}
