# Changelog

All notable changes to k3thrash are documented here. The format follows
[Keep a Changelog](https://keepachangelog.com/en/1.1.0/) and the project
adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [v0.1.0] — 2026-08-05

First release: the per-token NVMe expert-residency / reuse-ratio thrash
verdict for Kimi K3 (Moonshot AI 1.56 TB sparse-MoE, 896 experts / 16 active)
that `llama-bench` structurally cannot emit (it crashes on K3 and only gives
aggregate tps).

### m1 — sample + attach
- `k3thrash attach --pid <pid>` resolves the llama.cpp K3 pid, samples
  `/proc/<pid>/io.read_bytes` at 10 Hz, and prints a live `read_bytes/s` line
  to stdout.
- Raw samples are written to `trace.json` (atomic tmp-then-rename write).
- Linux-only procfs sampler; cross-platform stub keeps `go build ./...` green
  on macOS dev hosts.

### m2 — verdict correlate
- `--token-source PATH` tails llama.cpp stderr token-timing (fifo/file/stdin)
  and recovers the cumulative decode-token count.
- Per-token NVMe read-rate `Δread_bytes/Δtokens` is computed against the
  kimi-k3 topology (`16 active × ~1.6 GiB packed-expert` pathological
  baseline).
- Rolling one-line verdict (`healthy_warmup` / `partial_warm` /
  `pathological_thrash`) streams to stdout; ≥ 2× re-read rate ⇒ pathological.

### m3 — report render
- `k3thrash report trace.json` renders the shareable ASCII report: verdict
  one-liner + detail block + re-read-rate unicode sparkline.
- House-style README (`README.md` zh-primary + `README.en.md` sibling),
  animated hero/atlas dark-light SVGs, demo gif (`assets/demo/k3thrash-demo.gif`),
  goreleaser release workflow, vhs demo workflow.

### honesty note
- v0.1 granularity is **aggregate NVMe-read-rate** (per-token, summed across
  all experts, no expert identity). Per-expert identity via eBPF/uprobe is the
  v0.2 north star, deliberately out of scope.
