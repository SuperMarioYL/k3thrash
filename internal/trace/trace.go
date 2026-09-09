// Package trace records the per-token NVMe-read trace and exports it as JSON
// for pasting into the issue thread that reported the drifting-tps symptom.
package trace

import (
	"encoding/json"
	"os"
	"time"

	"github.com/SuperMarioYL/k3thrash/internal/thrash"
	"github.com/SuperMarioYL/k3thrash/internal/topo"
)

// Sample is one observation point in the trace: when it was taken, the
// cumulative /proc/<pid>/io.read_bytes at that instant, and the cumulative
// decode-token count the tokensource reported at the same instant.
type Sample struct {
	T             time.Time `json:"t"`
	ReadBytes     uint64    `json:"read_bytes"`
	TokensSoFar   uint64    `json:"tokens_so_far"`
	BytesPerToken float64   `json:"bytes_per_token,omitempty"` // derived Δread_bytes/Δtokens vs prev sample
}

// Trace is the exportable record of one attach session.
type Trace struct {
	Model        string         `json:"model"`
	Topo         topo.Topo      `json:"topo"`
	PID          int            `json:"pid"`
	StartedAt    time.Time      `json:"started_at"`
	EndedAt      time.Time      `json:"ended_at,omitempty"`
	Samples      []Sample       `json:"samples"`
	FinalVerdict *thrash.Verdict `json:"final_verdict,omitempty"`
}

// New constructs an empty Trace for the given model/pid/topo with a start ts.
func New(model string, pid int, t topo.Topo) *Trace {
	return &Trace{
		Model:     model,
		Topo:      t,
		PID:       pid,
		StartedAt: time.Now().UTC(),
		Samples:   make([]Sample, 0, 600), // ~1 min @10Hz
	}
}

// Append records a raw sample (T + ReadBytes + TokensSoFar) and back-fills the
// derived BytesPerToken delta against the previous sample. Returns the
// computed per-token rate (0 if this is the first sample or Δtokens == 0).
func (tr *Trace) Append(s Sample) float64 {
	if len(tr.Samples) > 0 {
		prev := tr.Samples[len(tr.Samples)-1]
		// Compare before subtracting: llama.cpp restarts per-prompt token
		// counting, so TokensSoFar can go backwards mid-session; a raw uint64
		// subtraction would underflow and yield a phantom near-zero rate that
		// biases the verdict toward healthy_warmup.
		if s.TokensSoFar > prev.TokensSoFar && s.ReadBytes >= prev.ReadBytes {
			s.BytesPerToken = float64(s.ReadBytes-prev.ReadBytes) /
				float64(s.TokensSoFar-prev.TokensSoFar)
		}
	}
	tr.Samples = append(tr.Samples, s)
	return s.BytesPerToken
}

// RateSeries returns the per-token read-rate series (BytesPerToken per sample,
// skipping leading zero entries that have no previous sample to delta against)
// suitable for sparkline rendering.
func (tr *Trace) RateSeries() []float64 {
	out := make([]float64, 0, len(tr.Samples))
	for _, s := range tr.Samples {
		if s.BytesPerToken > 0 {
			out = append(out, s.BytesPerToken)
		}
	}
	return out
}

// Finalize computes and stores the aggregate final verdict across all samples
// (mean per-token rate) and stamps the end time. Returns the verdict.
func (tr *Trace) Finalize() thrash.Verdict {
	tr.EndedAt = time.Now().UTC()
	mean := meanRate(tr.Samples)
	v := thrash.Judge(mean, tr.Topo)
	tr.FinalVerdict = &v
	return v
}

func meanRate(samples []Sample) float64 {
	var sum float64
	var n int
	for _, s := range samples {
		if s.BytesPerToken > 0 {
			sum += s.BytesPerToken
			n++
		}
	}
	if n == 0 {
		return 0
	}
	return sum / float64(n)
}

// WriteFile encodes the trace as indented JSON to path. Atomic-ish: writes to
// a sibling .tmp then renames, so a Ctrl-C mid-write never leaves a half JSON.
func (tr *Trace) WriteFile(path string) error {
	b, err := json.MarshalIndent(tr, "", "  ")
	if err != nil {
		return err
	}
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, b, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// ReadFile loads a trace from a JSON file (used by the report command).
func ReadFile(path string) (*Trace, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var tr Trace
	if err := json.Unmarshal(b, &tr); err != nil {
		return nil, err
	}
	return &tr, nil
}
