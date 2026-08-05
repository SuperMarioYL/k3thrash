package render

import (
	"fmt"
	"strings"

	"github.com/SuperMarioYL/k3thrash/internal/trace"
)

// Report renders the shareable ASCII block for a trace: a header with the
// model + run window, the final verdict one-liner, the verdict detail table,
// and a re-read-rate sparkline across the trace's per-token rate series.
// Designed to paste verbatim into an issue / forum thread.
func Report(tr *trace.Trace) string {
	var b strings.Builder
	b.WriteString("=== k3thrash report ===\n")
	fmt.Fprintf(&b, "model: %s   pid: %d   topo: %s (%d experts / %d active)\n",
		tr.Model, tr.PID, tr.Topo.Name, tr.Topo.NExperts, tr.Topo.NActive)
	fmt.Fprintf(&b, "window: %s → %s   samples: %d\n",
		tr.StartedAt.Format("15:04:05"), tr.EndedAt.Format("15:04:05"), len(tr.Samples))

	if tr.FinalVerdict != nil {
		b.WriteString("\nverdict:\n  ")
		b.WriteString(tr.FinalVerdict.OneLine())
		b.WriteString("\n  ")
		b.WriteString(tr.FinalVerdict.Summary())
		b.WriteString("\n")
	} else {
		b.WriteString("\nverdict: (not finalized — trace has no aggregate verdict)\n")
	}

	// Sparkline of per-token read rate shape.
	series := tr.RateSeries()
	if len(series) > 0 {
		b.WriteString("\nre-read-rate sparkline (bytes/token, min-max normalized):\n  ")
		b.WriteString(Sparkline(series))
		b.WriteString("\n  ")
		fmt.Fprintf(&b, "min %.0f | max %.0f B/token across %d samples\n",
			minFloat(series), maxFloat(series), len(series))
	}
	b.WriteString("\nshare: paste the block above into your thread / issue.\n")
	b.WriteString("=== end k3thrash report ===\n")
	return b.String()
}

func minFloat(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	m := xs[0]
	for _, x := range xs {
		if x < m {
			m = x
		}
	}
	return m
}

func maxFloat(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	m := xs[0]
	for _, x := range xs {
		if x > m {
			m = x
		}
	}
	return m
}
