package topo

import (
	"math"
	"testing"
)

func TestKimiK3(t *testing.T) {
	got := KimiK3()
	if got.Name != "kimi-k3" {
		t.Errorf("Name = %q, want kimi-k3", got.Name)
	}
	if got.NExperts != 896 {
		t.Errorf("NExperts = %d, want 896", got.NExperts)
	}
	if got.NActive != 16 {
		t.Errorf("NActive = %d, want 16", got.NActive)
	}
	if got.ExpertPackedBytes <= 0 {
		t.Errorf("ExpertPackedBytes = %d, want > 0", got.ExpertPackedBytes)
	}
	// ~1.6 GiB packed-4-bit: allow 1.4–1.8 GiB band.
	if got.ExpertPackedBytes < 1_400_000_000 || got.ExpertPackedBytes > 1_800_000_000 {
		t.Errorf("ExpertPackedBytes = %d, want within 1.4–1.8 GiB", got.ExpertPackedBytes)
	}
}

func TestLookup(t *testing.T) {
	cases := []string{"kimi-k3", "kimi_k3", "k3", "moonshot-ai/kimi-k3"}
	for _, c := range cases {
		if _, ok := Lookup(c); !ok {
			t.Errorf("Lookup(%q) = _, false, want true", c)
		}
	}
	if _, ok := Lookup("deepseek-v3"); ok {
		t.Error("Lookup(deepseek-v3) = true, want false (only kimi-k3 ships in v0.1)")
	}
}

func TestExpectedMinReadPerToken(t *testing.T) {
	tt := KimiK3()
	got := ExpectedMinReadPerToken(tt)
	want := float64(tt.NActive) * float64(tt.ExpertPackedBytes)
	if math.Abs(got-want) > 1 {
		t.Errorf("ExpectedMinReadPerToken = %g, want %g", got, want)
	}
	// sanity: 16 * ~1.6 GiB ≈ 25.7 GiB/token pathological baseline
	if got < 20_000_000_000 || got > 30_000_000_000 {
		t.Errorf("ExpectedMinReadPerToken = %g, want ~25 GiB/token", got)
	}
}
