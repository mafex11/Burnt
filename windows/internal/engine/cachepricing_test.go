package engine

import (
	"math"
	"testing"
)

func TestCacheSavingsIsInputMinusCacheReadRate(t *testing.T) {
	// 1,000,000 cache-read tokens on opus: input $15/Mtok, cache-read $1.50/Mtok.
	// Savings = (15.00 - 1.50) * 1.0 = $13.50.
	got := CacheSavings([]ModelBreakdown{{ModelName: "claude-opus-4-8", CacheReadTokens: 1_000_000}})
	if math.Abs(got-13.50) > 0.01 {
		t.Errorf("CacheSavings = %v, want 13.50", got)
	}
}

func TestCacheSavingsRatesPerFamily(t *testing.T) {
	cases := map[string]float64{
		"claude-opus-4-8":            13.50,
		"claude-sonnet-4-5-20250929": 2.70,
		"claude-haiku-4-5-20251001":  0.90,
	}
	for model, want := range cases {
		got := CacheSavings([]ModelBreakdown{{ModelName: model, CacheReadTokens: 1_000_000}})
		if math.Abs(got-want) > 0.01 {
			t.Errorf("CacheSavings(%s) = %v, want %v", model, got, want)
		}
	}
}

func TestCacheSavingsNonClaudeModelSavesNothing(t *testing.T) {
	got := CacheSavings([]ModelBreakdown{{ModelName: "gpt-5.4", CacheReadTokens: 1_000_000}})
	if got != 0 {
		t.Errorf("CacheSavings(gpt-5.4) = %v, want 0", got)
	}
}

func TestCacheSavingsZeroTokensSavesNothing(t *testing.T) {
	got := CacheSavings([]ModelBreakdown{{ModelName: "claude-opus-4-8", CacheReadTokens: 0}})
	if got != 0 {
		t.Errorf("CacheSavings with 0 cache reads = %v, want 0", got)
	}
}

func TestCacheSavingsUnknownClaudeFamilySavesNothing(t *testing.T) {
	got := CacheSavings([]ModelBreakdown{{ModelName: "claude-experimental-9", CacheReadTokens: 1_000_000}})
	if got != 0 {
		t.Errorf("CacheSavings for an unpriced Claude family = %v, want 0", got)
	}
}

func TestCacheSavingsSumsAcrossModels(t *testing.T) {
	got := CacheSavings([]ModelBreakdown{
		{ModelName: "claude-opus-4-8", CacheReadTokens: 1_000_000},
		{ModelName: "claude-haiku-4-5", CacheReadTokens: 1_000_000},
		{ModelName: "gpt-5.4", CacheReadTokens: 1_000_000},
	})
	if math.Abs(got-14.40) > 0.01 {
		t.Errorf("CacheSavings = %v, want 14.40", got)
	}
}
