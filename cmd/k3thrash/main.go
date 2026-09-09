// Command k3thrash is a Kimi K3 MoE expert-thrash diagnostic CLI. It attaches
// to a running llama.cpp K3 process, samples /proc/<pid>/io.read_bytes at
// 10 Hz, correlates it with the decode token rate, and emits a live per-token
// NVMe read-rate line + a rolling one-line thrash verdict (healthy_warmup /
// partial_warm / pathological_thrash). `k3thrash report trace.json` renders the
// shareable ASCII report (verdict + re-read-rate sparkline).
package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"
)

// Version is the build version; overridden at release via ldflags. It is the
// single source of truth displayed by `k3thrash --version`.
var Version = "v0.2.0"

func main() {
	if err := newRoot().Execute(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}

func newRoot() *cobra.Command {
	root := &cobra.Command{
		Use:   "k3thrash",
		Short: "Kimi K3 MoE expert-thrash NVMe diagnostic CLI",
		Long: `k3thrash — Kimi K3 MoE expert-residency / reuse-ratio thrash diagnostic.

Attach to a running llama.cpp K3 process and get a per-token NVMe read-rate
trace + a one-line thrash verdict (healthy_warmup / partial_warm /
pathological_thrash). v0.1 is aggregate NVMe-read-rate granularity; per-expert
identity via eBPF is the v0.2 north star.

Linux-only at the sampler layer (K3 NVMe rigs run Linux). The cross-platform
report command runs anywhere.`,
		SilenceUsage: true,
	}
	root.PersistentFlags().Bool("version", false, "print version and exit")
	root.AddCommand(newAttachCmd())
	root.AddCommand(newReportCmd())
	// Top-level `--version` short-circuit (so `k3thrash --version` works even
	// without a subcommand).
	root.RunE = func(cmd *cobra.Command, args []string) error {
		v, _ := cmd.Flags().GetBool("version")
		if v {
			fmt.Fprintln(cmd.OutOrStdout(), "k3thrash", Version)
			return nil
		}
		return cmd.Help()
	}
	return root
}
