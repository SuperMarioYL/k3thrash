package main

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/spf13/cobra"

	"github.com/SuperMarioYL/k3thrash/internal/procio"
	"github.com/SuperMarioYL/k3thrash/internal/thrash"
	"github.com/SuperMarioYL/k3thrash/internal/tokensource"
	"github.com/SuperMarioYL/k3thrash/internal/topo"
	"github.com/SuperMarioYL/k3thrash/internal/trace"
)

// newAttachCmd builds the `k3thrash attach` subcommand (m1 + m2 + m3): sample
// /proc/<pid>/io @10Hz, correlate with the tokensource, stream a live
// per-token NVMe read-rate line + rolling one-line verdict, write trace.json.
func newAttachCmd() *cobra.Command {
	var (
		pid          int
		expertTopo   string
		tokenSource  string
		outPath      string
		intervalMs   int
		noVerdict    bool
	)
	c := &cobra.Command{
		Use:   "attach --pid <pid> [--expert-topo kimi-k3] [--token-source PATH]",
		Short: "Attach to a running llama.cpp K3 pid and stream the thrash verdict",
		Long: `Attach to a running llama.cpp Kimi K3 process and stream a live per-token
NVMe read-rate line + rolling one-line thrash verdict. Writes raw + derived
samples to trace.json for the report command.

Linux-only (the sampler reads /proc/<pid>/io). Wire the tokensource to your
llama.cpp stderr, e.g. ` + "`" + `llama-cli ... 2> /tmp/k3.fifo` + "`" + ` then pass
--token-source /tmp/k3.fifo. If --token-source is empty, the verdict uses the
read-rate alone with the topo baseline (no per-token delta).`,
		RunE: func(cmd *cobra.Command, args []string) error {
			return runAttach(pid, expertTopo, tokenSource, outPath, intervalMs, noVerdict)
		},
	}
	c.Flags().IntVarP(&pid, "pid", "p", 0, "llama.cpp K3 process id to attach to (required)")
	c.Flags().StringVar(&expertTopo, "expert-topo", "kimi-k3", "MoE expert topology (v0.1: kimi-k3 only)")
	c.Flags().StringVar(&tokenSource, "token-source", "", "path to llama.cpp stderr token-timing stream (fifo/file), or '-' for stdin")
	c.Flags().StringVarP(&outPath, "out", "o", "trace.json", "output trace JSON path")
	c.Flags().IntVar(&intervalMs, "interval-ms", 100, "sample interval in ms (default 100 = 10Hz)")
	c.Flags().BoolVar(&noVerdict, "no-verdict", false, "stream raw read-bytes/s only, skip the verdict line")
	_ = c.MarkFlagRequired("pid")
	return c
}

func runAttach(pid int, expertTopoName, tokenSource, outPath string, intervalMs int, noVerdict bool) error {
	tt, ok := topo.Lookup(expertTopoName)
	if !ok {
		return fmt.Errorf("unknown --expert-topo %q (v0.1 ships only kimi-k3)", expertTopoName)
	}
	sampler, err := procio.New(pid, procio.Options{Interval: time.Duration(intervalMs) * time.Millisecond})
	if err != nil {
		return err
	}
	defer sampler.Close()

	var tl *tokensource.Tailer
	if tokenSource != "" {
		tl, err = tokensource.New(tokenSource)
		if err != nil {
			return err
		}
		defer tl.Wait()
	}

	interval := time.Duration(intervalMs) * time.Millisecond
	if interval <= 0 {
		interval = procio.DefaultOptions().Interval
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tr := trace.New(tt.Name, pid, tt)
	ticker := time.NewTicker(interval)
	defer ticker.Stop()

	// Rolling window for the live verdict (last 2s of samples → smooth jitter).
	const windowSize = 20 // 20 samples * 100ms = 2s
	window := make([]float64, 0, windowSize)

	fmt.Fprintf(os.Stderr, "k3thrash: attaching to pid %d (%s) @%dHz → %s\n",
		pid, tt.Name, 1000/intervalMs, outPath)
	fmt.Fprintln(os.Stderr, "  Ctrl-C to stop and write the final trace + verdict.")

	for {
		select {
		case <-ctx.Done():
			v := tr.Finalize()
			if err := tr.WriteFile(outPath); err != nil {
				return fmt.Errorf("write trace %s: %w", outPath, err)
			}
			fmt.Fprintf(os.Stderr, "\nk3thrash: wrote %s — %s\n", outPath, v.OneLine())
			return nil
		case <-ticker.C:
			s, err := sampler.Read()
			if err != nil {
				return fmt.Errorf("sampler read: %w", err)
			}
			tokens := uint64(0)
			if tl != nil {
				tokens = tl.CurrentTokens()
			}
			rate := tr.Append(trace.Sample{
				T:           s.T,
				ReadBytes:   s.ReadBytes,
				TokensSoFar: tokens,
			})
			// Live line: timestamp, read_bytes cumulative, rate B/tok.
			fmt.Printf("t=%s read=%d B", s.T.Format("15:04:05.000"), s.ReadBytes)
			if rate > 0 {
				fmt.Printf(" rate=%.0f B/tok", rate)
			}
			if !noVerdict && rate > 0 {
				window = append(window, rate)
				if len(window) > windowSize {
					window = window[1:]
				}
				mean := meanFloat(window)
				v := thrash.Judge(mean, tt)
				fmt.Printf("  %s", v.OneLine())
			}
			fmt.Println()
		}
	}
}

func meanFloat(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	var sum float64
	for _, x := range xs {
		sum += x
	}
	return sum / float64(len(xs))
}
