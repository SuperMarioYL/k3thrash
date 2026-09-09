package main

import (
	"bytes"
	"context"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/SuperMarioYL/k3thrash/internal/procio"
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

// fakeSampler feeds a fixed sequence of IOSamples then a terminal error, so
// the pid-death path (sampler failing mid-attach) is testable on any host.
type fakeSampler struct {
	samples []procio.IOSample
	err     error
	n       int
}

func (f *fakeSampler) Read() (procio.IOSample, error) {
	if f.n < len(f.samples) {
		s := f.samples[f.n]
		f.n++
		return s, nil
	}
	return procio.IOSample{}, f.err
}

func (*fakeSampler) Close() error { return nil }

func TestSampleInterval(t *testing.T) {
	cases := []struct {
		ms      int
		wantDur time.Duration
		wantHz  int
	}{
		{0, 100 * time.Millisecond, 10}, // v0.1.0 divided by zero here
		{-200, 100 * time.Millisecond, 10},
		{50, 50 * time.Millisecond, 20},
		{100, 100 * time.Millisecond, 10},
		{250, 250 * time.Millisecond, 4},
		{1000, time.Second, 1},
	}
	for _, c := range cases {
		gotDur, gotHz := sampleInterval(c.ms)
		if gotDur != c.wantDur || gotHz != c.wantHz {
			t.Errorf("sampleInterval(%d) = (%v, %d Hz), want (%v, %d Hz)",
				c.ms, gotDur, gotHz, c.wantDur, c.wantHz)
		}
	}
}

func TestSampleLoopPersistsTraceOnSamplerError(t *testing.T) {
	// The K3 pid dying mid-attach (sampler read error) must still write the
	// captured trace — v0.1.0 returned the error and lost the whole session.
	dir := t.TempDir()
	out := filepath.Join(dir, "trace.json")
	now := time.Now().UTC()
	fs := &fakeSampler{
		samples: []procio.IOSample{
			{T: now, ReadBytes: 0},
			{T: now.Add(10 * time.Millisecond), ReadBytes: 3_000_000_000},
			{T: now.Add(20 * time.Millisecond), ReadBytes: 6_000_000_000},
		},
		err: errors.New("pid gone"),
	}
	tr := trace.New("kimi-k3", 123, topo.KimiK3())

	err := sampleLoop(context.Background(), fs, nil, tr, out, 5*time.Millisecond, false)
	if err == nil || !strings.Contains(err.Error(), "pid gone") {
		t.Fatalf("err = %v, want the sampler error surfaced", err)
	}

	loaded, rerr := trace.ReadFile(out)
	if rerr != nil {
		t.Fatalf("trace was not persisted on pid death: %v", rerr)
	}
	if len(loaded.Samples) != 3 {
		t.Errorf("persisted samples = %d, want 3", len(loaded.Samples))
	}
	if loaded.FinalVerdict == nil {
		t.Error("persisted trace has no final verdict")
	}
}

func TestSampleLoopPersistsTraceOnCancel(t *testing.T) {
	// Ctrl-C (ctx cancel) must write the trace and return nil.
	dir := t.TempDir()
	out := filepath.Join(dir, "trace.json")
	now := time.Now().UTC()
	samples := make([]procio.IOSample, 0, 100)
	for i := 0; i < 100; i++ {
		samples = append(samples, procio.IOSample{T: now.Add(time.Duration(i) * 5 * time.Millisecond), ReadBytes: uint64(i) * 1_000})
	}
	fs := &fakeSampler{samples: samples, err: errors.New("never reached")}
	ctx, cancel := context.WithCancel(context.Background())
	tr := trace.New("kimi-k3", 321, topo.KimiK3())

	go func() {
		time.Sleep(25 * time.Millisecond)
		cancel()
	}()
	if err := sampleLoop(ctx, fs, nil, tr, out, 5*time.Millisecond, false); err != nil {
		t.Fatalf("sampleLoop on cancel: %v", err)
	}
	loaded, rerr := trace.ReadFile(out)
	if rerr != nil {
		t.Fatalf("trace was not persisted on cancel: %v", rerr)
	}
	if len(loaded.Samples) < 2 {
		t.Errorf("persisted samples = %d, want at least 2 captured before cancel", len(loaded.Samples))
	}
	if loaded.FinalVerdict == nil {
		t.Error("persisted trace has no final verdict")
	}
}
