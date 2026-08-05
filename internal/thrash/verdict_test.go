package thrash

import (
	"math"
	"testing"

	"github.com/SuperMarioYL/k3thrash/internal/topo"
)

func TestJudge(t *testing.T) {
	tt := topo.KimiK3()
	baseline := topo.ExpectedMinReadPerToken(tt) // ~25.7 GiB/token

	cases := []struct {
		name        string
		actual      float64
		wantClass   string
		wantRereadX float64 // approximate
	}{
		{"zero_read_resident", 0, HealthyWarmup, 0},
		{"half_baseline_healthy", baseline * 0.5, HealthyWarmup, 0.5},
		{"near_baseline_0p9_healthy", baseline * 0.9, HealthyWarmup, 0.9},
		{"boundary_1p0_partial", baseline * 1.0, PartialWarm, 1.0},
		{"1p5_partial", baseline * 1.5, PartialWarm, 1.5},
		{"2p0_pathological", baseline * 2.0, PathologicalThrash, 2.0},
		{"3p2_pathological", baseline * 3.2, PathologicalThrash, 3.2},
		{"10x_pathological", baseline * 10.0, PathologicalThrash, 10.0},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			v := Judge(c.actual, tt)
			if v.Classification != c.wantClass {
				t.Errorf("classification = %q, want %q", v.Classification, c.wantClass)
			}
			if math.Abs(v.RereadRateX-c.wantRereadX) > 0.01 {
				t.Errorf("reread_rate_x = %.4f, want %.4f", v.RereadRateX, c.wantRereadX)
			}
			// inverse relationship
			if v.RereadRateX > 0 {
				wantReuse := 1 / v.RereadRateX
				if math.Abs(v.ReuseRatio-wantReuse) > 0.01 {
					t.Errorf("reuse_ratio = %.4f, want %.4f (1/reread_rate_x)", v.ReuseRatio, wantReuse)
				}
			}
		})
	}
}

func TestVerdictOneLine(t *testing.T) {
	tt := topo.KimiK3()
	baseline := topo.ExpectedMinReadPerToken(tt)

	v := Judge(baseline*3.2, tt)
	got := v.OneLine()
	if want := "re-read rate 3.2× reuse: pathological thrash"; got != want {
		t.Errorf("OneLine() = %q, want %q", got, want)
	}

	v2 := Judge(baseline*0.9, tt)
	got2 := v2.OneLine()
	if want := "re-read rate 0.9× reuse: healthy warmup"; got2 != want {
		t.Errorf("OneLine() = %q, want %q", got2, want)
	}
}

func TestJudgeDegenerate(t *testing.T) {
	// Empty topology: baseline 0 → verdict should be healthy, not panic.
	empty := topo.Topo{}
	v := Judge(12345.0, empty)
	if v.Classification != HealthyWarmup {
		t.Errorf("degenerate topo classification = %q, want %q", v.Classification, HealthyWarmup)
	}
	if v.RereadRateX != 0 {
		t.Errorf("degenerate topo reread_rate_x = %v, want 0", v.RereadRateX)
	}
}

func TestSummaryNonEmpty(t *testing.T) {
	tt := topo.KimiK3()
	v := Judge(topo.ExpectedMinReadPerToken(tt)*2.5, tt)
	if v.Summary() == "" {
		t.Error("Summary() returned empty string")
	}
}
