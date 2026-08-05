// Package thrash computes the per-token expert-residency / reuse-ratio thrash
// verdict — the derived signal that aggregate-tps benchmarks (llama-bench)
// structurally cannot emit. The verdict is a printable one-liner an operator
// pastes into the issue thread that reported the drifting-tps symptom.
//
// v0.1 granularity is honest: aggregate NVMe read-rate summed across all
// experts, no per-expert identity. Per-expert identity via eBPF/uprobe is the
// v0.2 north star (deliberately out of scope here).
package thrash

import (
	"fmt"
	"math"

	"github.com/SuperMarioYL/k3thrash/internal/topo"
)

// Verdict is the printable judgement of whether the observed NVMe read-rate
// looks like healthy expert warmup, partial warming, or pathological
// expert-re-read thrash.
type Verdict struct {
	// ReuseRatio = ExpectedMinReadPerToken / actualReadPerToken.
	// >1 means the bus is reading less than the all-active-re-read baseline
	// (experts are being reused); <1 means the bus is re-reading faster than
	// reuse (thrash).
	ReuseRatio float64 `json:"reuse_ratio"`
	// RereadRateX = 1 / ReuseRatio — "re-read rate times reuse".
	// <1 = healthy; ~1 = boundary; >2 = pathological.
	RereadRateX float64 `json:"reread_rate_x"`
	// Classification is the human-readable bucket.
	Classification string `json:"classification"`
	// NvmeBytesPerToken is the measured Δread_bytes/Δtokens feeding the verdict.
	NvmeBytesPerToken float64 `json:"nvme_bytes_per_token"`
	// ExpectedMinReadPerToken is the topo-derived all-active-re-read baseline.
	ExpectedMinReadPerToken float64 `json:"expected_min_read_per_token"`
}

// Classification buckets.
const (
	HealthyWarmup      = "healthy_warmup"
	PartialWarm        = "partial_warm"
	PathologicalThrash = "pathological_thrash"
)

// PathologicalThreshold is the re-read rate above which the run is flagged as
// pathological thrash (per mvp_plan §2: "A rate above ~2× reuse ⇒
// pathological"). Below 1.0 is healthy; 1.0–2.0 is partial-warm.
const PathologicalThreshold = 2.0

// Judge computes a Verdict from the measured NVMe bytes-per-token rate against
// the supplied MoE topology. actualReadPerToken is Δread_bytes / Δtokens
// (zero is valid — fully resident, no NVMe reads).
func Judge(actualReadPerToken float64, t topo.Topo) Verdict {
	expectedMin := topo.ExpectedMinReadPerToken(t)
	v := Verdict{
		ExpectedMinReadPerToken: expectedMin,
		NvmeBytesPerToken:       actualReadPerToken,
	}
	switch {
	case expectedMin <= 0:
		// Degenerate topology: cannot compute a baseline. Treat as healthy
		// (no baseline to exceed) with infinite reuse ratio.
		v.ReuseRatio = math.Inf(1)
		v.RereadRateX = 0
		v.Classification = HealthyWarmup
	case actualReadPerToken <= 0:
		// No NVMe reads at all — experts fully resident.
		v.ReuseRatio = math.Inf(1)
		v.RereadRateX = 0
		v.Classification = HealthyWarmup
	default:
		v.ReuseRatio = expectedMin / actualReadPerToken
		v.RereadRateX = actualReadPerToken / expectedMin // = 1/ReuseRatio
		v.Classification = classify(v.RereadRateX)
	}
	return v
}

func classify(rereadRateX float64) string {
	switch {
	case rereadRateX >= PathologicalThreshold:
		return PathologicalThrash
	case rereadRateX >= 1.0:
		return PartialWarm
	default:
		return HealthyWarmup
	}
}

// OneLine renders the rolling one-line verdict an operator reads live and
// pastes into an issue thread, e.g.:
//
//	re-read rate 3.2× reuse: pathological thrash
//	re-read rate 0.9× reuse: healthy warmup
//
// The "× reuse" suffix echoes the mvp_plan readme pitch so the verdict an
// operator shares matches the README's framing verbatim.
func (v Verdict) OneLine() string {
	return fmt.Sprintf("re-read rate %.1f× reuse: %s", v.RereadRateX, prettify(v.Classification))
}

func prettify(c string) string {
	switch c {
	case HealthyWarmup:
		return "healthy warmup"
	case PartialWarm:
		return "partial warm"
	case PathologicalThrash:
		return "pathological thrash"
	}
	return c
}

// Summary renders the multi-line block used at the top of the ASCII report.
func (v Verdict) Summary() string {
	return fmt.Sprintf(
		"re-read rate %.2f× reuse | reuse ratio %.2f | classification: %s\nnvme read %.2f B/token (pathological baseline %.2f B/token)",
		v.RereadRateX, v.ReuseRatio, prettify(v.Classification),
		v.NvmeBytesPerToken, v.ExpectedMinReadPerToken,
	)
}
