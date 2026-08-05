package main

import (
	"bytes"
	"context"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SuperMarioYL/k3thrash/internal/topo"
	"github.com/SuperMarioYL/k3thrash/internal/trace"
)

// writeExampleTrace writes a small finalized trace to path so report tests +
// the demo can consume it without a live K3 pid.
func writeExampleTrace(t *testing.T, path string) {
	t.Helper()
	tt := topo.KimiK3()
	tr := trace.New("kimi-k3", 4242, tt)
	now := time.Now().UTC().Truncate(time.Second)
	tr.StartedAt = now
	baseline := 25_000_000_000.0
	// Simulate a pathological-thrash run: 3× baseline per token.
	tr.Append(trace.Sample{T: now, ReadBytes: 0, TokensSoFar: 0})
	tr.Append(trace.Sample{T: now.Add(100 * time.Millisecond), ReadBytes: uint64(3 * baseline), TokensSoFar: 1})
	tr.Append(trace.Sample{T: now.Add(200 * time.Millisecond), ReadBytes: uint64(6 * baseline), TokensSoFar: 2})
	tr.Append(trace.Sample{T: now.Add(300 * time.Millisecond), ReadBytes: uint64(9 * baseline), TokensSoFar: 3})
	tr.Append(trace.Sample{T: now.Add(400 * time.Millisecond), ReadBytes: uint64(12 * baseline), TokensSoFar: 4})
	tr.Finalize()
	if err := tr.WriteFile(path); err != nil {
		t.Fatalf("write trace: %v", err)
	}
}

func TestReportCommand(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "trace.json")
	writeExampleTrace(t, path)

	cmd := newReportCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetArgs([]string{path})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("report command: %v", err)
	}
	got := out.String()
	for _, want := range []string{
		"k3thrash report",
		"verdict",
		"re-read rate",
		"pathological thrash",
		"sparkline",
	} {
		if !strings.Contains(got, want) {
			t.Errorf("report output missing %q\n--- output ---\n%s", want, got)
		}
	}
	if !strings.ContainsAny(got, "▁▂▃▄▅▆▇█") {
		t.Errorf("report output has no sparkline runes\n%s", got)
	}
}

func TestReportCommandFinalizesUnfinalized(t *testing.T) {
	// A trace that was never Finalize()d should still produce a report
	// (best-effort verdict), not a panic / empty block.
	dir := t.TempDir()
	path := filepath.Join(dir, "raw.json")
	tt := topo.KimiK3()
	tr := trace.New("kimi-k3", 1, tt)
	now := time.Now().UTC().Truncate(time.Second)
	tr.StartedAt = now
	tr.Append(trace.Sample{T: now, ReadBytes: 0, TokensSoFar: 0})
	tr.Append(trace.Sample{T: now.Add(time.Second), ReadBytes: 1_610_612_736, TokensSoFar: 1})
	if err := tr.WriteFile(path); err != nil {
		t.Fatalf("write: %v", err)
	}

	cmd := newReportCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{path})
	if err := cmd.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("report command: %v", err)
	}
	if !strings.Contains(out.String(), "k3thrash report") {
		t.Errorf("unfinalized report did not render\n%s", out.String())
	}
}

func TestReportCommandBadPath(t *testing.T) {
	cmd := newReportCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"/nonexistent/trace.json"})
	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("expected error for missing trace file, got nil")
	}
}

func TestRootVersion(t *testing.T) {
	root := newRoot()
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--version"})
	if err := root.ExecuteContext(context.Background()); err != nil {
		t.Fatalf("root --version: %v", err)
	}
	if !strings.Contains(out.String(), "k3thrash") {
		t.Errorf("version output = %q, want k3thrash version", out.String())
	}
}

func TestAttachRejectsBadTopo(t *testing.T) {
	// Cross-platform: bad topo name fails before the Linux-only sampler is built.
	cmd := newAttachCmd()
	cmd.SetOut(io.Discard)
	cmd.SetErr(io.Discard)
	cmd.SetArgs([]string{"--pid", "1", "--expert-topo", "deepseek-v3"})
	err := cmd.ExecuteContext(context.Background())
	if err == nil {
		t.Fatal("expected error for unknown --expert-topo, got nil")
	}
	if !strings.Contains(err.Error(), "deepseek-v3") {
		t.Errorf("error should name the bad topo: %v", err)
	}
}

// Ensure the cobra root wires subcommands without panic.
func TestRootHasSubcommands(t *testing.T) {
	root := newRoot()
	subs := root.Commands()
	names := make(map[string]bool)
	for _, c := range subs {
		names[c.Name()] = true
	}
	for _, want := range []string{"attach", "report"} {
		if !names[want] {
			t.Errorf("root missing %q subcommand", want)
		}
	}
}

var _ = os.Stdout // keep os imported if future tests need it
