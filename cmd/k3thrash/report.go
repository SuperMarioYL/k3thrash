package main

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/SuperMarioYL/k3thrash/internal/render"
	"github.com/SuperMarioYL/k3thrash/internal/trace"
)

// newReportCmd builds `k3thrash report <trace.json>` (m3): render the
// shareable ASCII report (verdict + re-read-rate sparkline). Cross-platform.
func newReportCmd() *cobra.Command {
	c := &cobra.Command{
		Use:   "report <trace.json>",
		Short: "Render the shareable ASCII report (verdict + sparkline) for a trace",
		Long: `Render the shareable ASCII report for a saved trace.json — the final
verdict one-liner, the verdict detail block, and a re-read-rate sparkline
across the per-token read-rate series. Paste the block into an
r/LocalLLaMA / 掘金 / V2EX thread to share the diagnosis.`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			path := args[0]
			tr, err := trace.ReadFile(path)
			if err != nil {
				return fmt.Errorf("read trace %s: %w", path, err)
			}
			// If the trace was never finalized (e.g. attach crashed), compute a
			// best-effort verdict now so the report still renders something.
			if tr.FinalVerdict == nil {
				v := tr.Finalize()
				_ = v
			}
			fmt.Fprint(cmd.OutOrStdout(), render.Report(tr))
			return nil
		},
	}
	return c
}
