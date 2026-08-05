// Package topo carries MoE expert-topology constants used by the thrash
// verdict engine. Only kimi-k3 ships in v0.1; the registry is pluggable for
// future DeepSeek-V3.x / Qwen3-MoE support (see roadmap).
package topo

// Topo describes a sparse-MoE checkpoint's expert layout. The verdict engine
// only needs three numbers: how many experts exist, how many fire per token,
// and how big one packed expert is on the NVMe bus.
type Topo struct {
	Name              string `json:"name"`
	NExperts          int    `json:"n_experts"`            // total routed experts (kimi-k3: 896)
	NActive           int    `json:"n_active"`             // experts active per token (kimi-k3: 16)
	ExpertPackedBytes int64  `json:"expert_packed_bytes"`  // size of one packed expert on disk (kimi-k3: ~1.6 GiB packed-4-bit)
}

// KimiK3 returns the topology for Moonshot AI Kimi K3 (1.56 TB sparse-MoE,
// 896 experts / 16 active, packed-4-bit experts streamed off NVMe with no
// dequantization step). ExpertPackedBytes is the ~1.6 GiB packed-4-bit size
// observed by home-lab operators streaming the checkpoint off NVMe.
func KimiK3() Topo {
	const kimiK3ExpertPackedBytes int64 = 1_610_612_736 // ~1.6 GiB (1.5 GiB + headroom)
	return Topo{
		Name:              "kimi-k3",
		NExperts:          896,
		NActive:           16,
		ExpertPackedBytes: kimiK3ExpertPackedBytes,
	}
}

// Lookup returns the named topology. v0.1 only knows "kimi-k3" (and the
// aliases "kimi_k3"/"k3"); unknown names return ok=false so the caller can
// surface a clear error rather than silently falling back.
func Lookup(name string) (Topo, bool) {
	switch name {
	case "kimi-k3", "kimi_k3", "k3", "moonshot-ai/kimi-k3":
		return KimiK3(), true
	}
	return Topo{}, false
}

// ExpectedMinReadPerToken is the bytes-per-token baseline the thrash engine
// compares the measured NVMe read-rate against. It is the read rate you would
// observe if every active expert were re-read off NVMe on every single token
// (the pathological all-active-re-read baseline). When experts are resident
// the measured read rate drops below this baseline (reuse ratio > 1); when
// experts are being re-read faster than they are reused the measured rate
// climbs above it (reuse ratio < 1, re-read rate > 1x).
//
// Per token the bus carries NActive packed experts, so:
//
//	ExpectedMinReadPerToken = NActive * ExpertPackedBytes
func ExpectedMinReadPerToken(t Topo) float64 {
	return float64(t.NActive) * float64(t.ExpertPackedBytes)
}
