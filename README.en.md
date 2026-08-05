<div align="right"><sub><b>English</b> &nbsp;·&nbsp; [<a href="./README.md">简体中文</a>]</sub></div>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./assets/hero-dark.svg">
    <source media="(prefers-color-scheme: light)" srcset="./assets/hero-light.svg">
    <img src="./assets/hero-light.svg" width="880" alt="k3thrash — Kimi K3 MoE expert-thrash NVMe diagnostic CLI">
  </picture>
</p>

<p align="center"><sub>Attach to a running llama.cpp K3 process and get a per-token NVMe re-read/reuse ratio + a one-line thrash verdict.</sub></p>

<p align="center">
  <a href="./LICENSE"><img src="https://img.shields.io/badge/license-MIT-blue" alt="license MIT"></a>
  <a href="https://github.com/SuperMarioYL/k3thrash/releases"><img src="https://img.shields.io/github/v/release/SuperMarioYL/k3thrash?label=release" alt="latest release"></a>
  <img src="https://img.shields.io/github/actions/workflow/status/SuperMarioYL/k3thrash/ci.yml?branch=main&label=CI" alt="CI">
  <img src="https://img.shields.io/badge/Go-1.24-0071E3?logo=go&logoColor=white" alt="Go 1.24">
  <img src="https://img.shields.io/badge/kimi--k3-896e%2F16a-5E5CE6" alt="kimi-k3">
  <img src="https://img.shields.io/badge/MoE-thrash%20diagnostic-10A37F" alt="MoE thrash diagnostic">
</p>

**Kimi K3 decode tps drifts upward over time — is it healthy cache warmup, or pathological expert re-reads thrashing the NVMe bus? k3thrash attaches and tells you in one line.**

<h2><img src="https://api.iconify.design/tabler:topology-star-3.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> Architecture</h2>

<p align="center">
  <picture>
    <source media="(prefers-color-scheme: dark)" srcset="./assets/atlas-dark.svg">
    <source media="(prefers-color-scheme: light)" srcset="./assets/atlas-light.svg">
    <img src="./assets/atlas-light.svg" width="880" alt="architecture: llama.cpp K3 pid → procio+tokensource → thrash verdict → trace.json + report">
  </picture>
</p>

Two processes: your running `llama.cpp` Kimi K3 pid plus the `k3thrash` CLI. No llama.cpp recompile, no model reload. `procio` samples `/proc/<pid>/io.read_bytes` at 10 Hz; `tokensource` tails llama.cpp stderr per-token timing; the `thrash` engine computes the per-token NVMe re-read rate against the kimi-k3 topology (896 experts / 16 active / ~1.6 GiB packed expert); `render` emits the shareable ASCII report + sparkline.

<h2><img src="https://api.iconify.design/tabler:bulb.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> Why this exists</h2>

Kimi K3 (Moonshot AI's open-weights 1.56 TB sparse-MoE, 896 experts / 16 active) only runs on a home NVMe rig by streaming experts off the drive on demand — 93% of the checkpoint is routed experts that never become resident. That produces a counterintuitive failure mode: decoding tokens-per-second *drifts upward over time* like a "warmup," but you can't tell whether you're watching healthy cache warming, host page-cache effects, or pathological expert re-reads hammering the NVMe bus. `llama-bench` crashes on K3 outright, and even when it runs it only gives aggregate tps — it structurally cannot decompose the drift into per-token expert-residency attribution. k3thrash makes that verdict a pasteable one-liner.

| Axis | `llama-bench` | `xpref` | **k3thrash** |
|---|---|---|---|
| Per-token NVMe re-read rate | — | — | ✓ |
| Thrash verdict (healthy/partial/pathological) | — | — | ✓ |
| Shareable ASCII report + sparkline | — | partial | ✓ |
| Does not crash on K3 | ✗ | ✓ | ✓ |
| Tells you *whether* to adopt xpref | — | — | ✓ |
| Improves throughput (prefetch) | — | ✓ | ✗ (v0.1 only diagnoses) |

Honesty note: v0.1 granularity is **aggregate NVMe read-rate** (per-token, summed across all experts, no expert identity). Per-expert identity (which of 896 fired) via eBPF/uprobe is the v0.2 north star, deliberately out of scope here.

<h2><img src="https://api.iconify.design/tabler:rocket.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> Quickstart</h2>

```bash
go install github.com/SuperMarioYL/k3thrash@latest          # 1. install
k3thrash report examples/trace.example.json                 # 2. see the one-line verdict (cross-platform)
# on a Linux K3 rig: k3thrash attach --pid $(pgrep -f llama)  # 3. attach to a real K3 pid, verdict in ~30s
```

<details>
<summary>sample output (<code>k3thrash report examples/trace.example.json</code>)</summary>

```
=== k3thrash report ===
model: kimi-k3   pid: 4242   topo: kimi-k3 (896 experts / 16 active)
window: 13:45:00 → 13:45:25   samples: 6

verdict:
  re-read rate 3.0× reuse: pathological thrash
  re-read rate 3.00× reuse | reuse ratio 0.33 | classification: pathological thrash
nvme read 77309411328.00 B/token (pathological baseline 25769797776.00 B/token)

re-read-rate sparkline (bytes/token, min-max normalized):
  ▅▅▅▅▅
  min 77309411328 | max 77309411328 B/token across 5 samples

share: paste the block above into your thread / issue.
=== end k3thrash report ===
```
</details>

<h2><img src="https://api.iconify.design/tabler:terminal-2.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> Usage</h2>

**attach — attach to a running llama.cpp K3 pid, stream the verdict + write trace.json (Linux only)**

```bash
# route llama.cpp stderr into a fifo so k3thrash can read token counts too
mkfifo /tmp/k3.fifo
llama-cli -m kimi-k3.gguf ... 2> /tmp/k3.fifo &

k3thrash attach --pid $(pgrep -f llama) --expert-topo kimi-k3 \
  --token-source /tmp/k3.fifo --out trace.json
# Ctrl-C to stop → writes trace.json + a one-line final verdict
```

**report — render the shareable ASCII report from trace.json (cross-platform, runs on macOS too)**

```bash
k3thrash report trace.json            # print verdict + sparkline
k3thrash report trace.json > share.txt  # redirect to a file for the forum
```

**version**

```bash
k3thrash --version                    # k3thrash v0.1.0
```

<h2><img src="https://api.iconify.design/tabler:photo.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> Demo</h2>

<p align="center"><img src="./assets/demo/k3thrash-demo.gif" width="880" alt="k3thrash demo: report renders verdict + sparkline"></p>

The demo gif is rendered from [`docs/demo.tape`](./docs/demo.tape) (a vhs script) by `.github/workflows/demo.yml` — trigger it manually to refresh. The committed gif is the source of truth.

<h2><img src="https://api.iconify.design/tabler:adjustments.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> Configuration</h2>

Key `k3thrash attach` flags:

| flag | type | default | meaning |
|---|---|---|---|
| `--pid` / `-p` | int | (required) | llama.cpp K3 process id to attach to |
| `--expert-topo` | string | `kimi-k3` | MoE expert topology (v0.1: `kimi-k3` only) |
| `--token-source` | string | `""` | path to llama.cpp stderr token-timing stream (fifo/file), `-` for stdin; empty = use read-rate + topo baseline only |
| `--out` / `-o` | string | `trace.json` | trace JSON output path |
| `--interval-ms` | int | `100` | sample interval ms (default 100 = 10 Hz) |
| `--no-verdict` | bool | `false` | stream raw read_bytes/s only, skip the verdict line |

Verdict thresholds (`internal/thrash/verdict.go`): re-read rate `< 1.0×` → `healthy_warmup`; `1.0–2.0×` → `partial_warm`; `≥ 2.0×` → `pathological_thrash`.

<h2><img src="https://api.iconify.design/tabler:currency-yuan.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> Pricing</h2>

**Free forever for individual self-hosters** (this repo is MIT). Multi-node K3 / multi-MoE-rig tuning labs need cross-node comparison, historical traces, and threshold alerting — that's the post-MVP hosted fleet-dashboard tier (explicitly out of scope per `mvp_plan §6`, not in this repo).

- First paying customer: labs with ≥2-node NVMe rigs tuning CN MoE, university AI-infra groups, startups running K3 + DeepSeek-V3.x.
- Price (estimate): ¥299/rig/month (~$40/rig/mo), per-rig not per-seat.
- Minimum "card-swipe" demo path: invite-only PoC — the lab uploads 3 multi-node K3 run `trace.json` files, sees cross-node re-read-rate comparison + alert-threshold config page; the moment their own pathological node is flagged red = the payment trigger.
- Want the fleet tier? Drop your node count + scenario in [Discussions](https://github.com/SuperMarioYL/k3thrash/discussions) and you're first in line when the PoC opens.

<h2><img src="https://api.iconify.design/tabler:map-2.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> Roadmap</h2>

- [x] **m1** sample `/proc/<pid>/io` @10Hz, stream `read_bytes/s` + write `trace.json`
- [x] **m2** correlate decode token rate, compute reuse-ratio vs kimi-k3 topo, emit one-line verdict
- [x] **m3** `k3thrash report` renders ASCII report + sparkline; README + demo gif + release CI
- [ ] **v0.2** per-expert identity signal via eBPF/uprobe (which of 896 fired per token) — north star
- [ ] Pluggable topologies: DeepSeek-V3.x / Qwen3-MoE and other NVMe-demand sparse-MoE
- [ ] Hosted fleet-dashboard tier (multi-node comparison + historical traces + threshold alerting, post-MVP commercial tier)
- [ ] Real-time alerting / notifications

<h2><img src="https://api.iconify.design/tabler:license.svg?color=%230071E3&width=24" height="22" align="absmiddle" alt=""> License & Contributing</h2>

MIT, see [LICENSE](./LICENSE). File bugs / feature requests in [Issues](https://github.com/SuperMarioYL/k3thrash/issues); PRs welcome — fork → branch → PR. CN users can use the [Gitee mirror](https://gitee.com/SuperMarioYL/k3thrash) (synced by the maintainer after push).

<p align="center"><sub><a href="./LICENSE">MIT</a> © 2026 SuperMarioYL</sub></p>
