package engine

import "strings"

// cacheRate is (input, cacheRead) USD per million tokens for a model-name prefix.
type cacheRate struct {
	prefix    string
	input     float64
	cacheRead float64
}

// cacheRateTable holds representative LiteLLM USD-per-million-token values,
// hardcoded because cache savings is an illustrative "you saved ~$Y" figure, not
// billing. First matching prefix wins.
var cacheRateTable = []cacheRate{
	{"claude-opus", 15.0, 1.50},
	{"claude-sonnet", 3.0, 0.30},
	{"claude-haiku", 1.0, 0.10},
}

// CacheSavings estimates the dollars saved by prompt caching across the given
// model breakdowns. Claude-only and approximate.
func CacheSavings(models []ModelBreakdown) float64 {
	total := 0.0
	for _, m := range models {
		total += cacheSavingsForModel(m.CacheReadTokens, m.ModelName)
	}
	return total
}

// cacheSavingsForModel is the per-model estimate: cache-read tokens billed at the
// cache-read rate instead of the full input rate.
func cacheSavingsForModel(cacheReadTokens int64, model string) float64 {
	if ClassifyModel(model) != ToolClaude || cacheReadTokens <= 0 {
		return 0
	}
	name := strings.ToLower(model)
	for _, r := range cacheRateTable {
		if strings.HasPrefix(name, r.prefix) {
			return float64(cacheReadTokens) * (r.input - r.cacheRead) / 1_000_000.0
		}
	}
	return 0
}
