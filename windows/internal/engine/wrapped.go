package engine

import "sort"

// Wrapped is the "year in review" card's raw numbers, formatted by the UI.
type Wrapped struct {
	MonthCost   float64        `json:"monthCost"`
	AllTimeCost float64        `json:"allTimeCost"`
	TopModels   []WrappedModel `json:"topModels"`
	BusiestDay  DayPoint       `json:"busiestDay"`
	ClaudeShare float64        `json:"claudeShare"` // 0...1 of week cost
	CacheSaved  float64        `json:"cacheSaved"`
}

// WrappedModel is one bar of the Wrapped card's model chart.
type WrappedModel struct {
	Model string  `json:"model"`
	Cost  float64 `json:"cost"`
}

// wrappedTopModels is how many model bars the card shows.
const wrappedTopModels = 5

// buildWrapped derives the Wrapped card from the already-aggregated pieces.
// busiestDay is the highest-cost day of the heatmap window; ties go to the oldest,
// matching the Swift `max(by:)` semantics.
func buildWrapped(month, allTime Totals, byModel []ModelSlice, byTool []ToolSlice,
	heatmap []DayPoint, cacheSavings float64) Wrapped {
	top := make([]WrappedModel, 0, wrappedTopModels)
	for _, m := range byModel {
		if len(top) == wrappedTopModels {
			break
		}
		top = append(top, WrappedModel{Model: m.Model, Cost: m.Cost})
	}

	var busiest DayPoint
	for i, p := range heatmap {
		if i == 0 || p.Cost > busiest.Cost {
			busiest = p
		}
	}

	claudeCost, toolCost := 0.0, 0.0
	for _, t := range byTool {
		toolCost += t.Cost
		if t.Tool == ToolClaude {
			claudeCost += t.Cost
		}
	}
	share := 0.0
	if toolCost > 0 {
		share = claudeCost / toolCost
	}

	return Wrapped{
		MonthCost:   month.Cost,
		AllTimeCost: allTime.Cost,
		TopModels:   top,
		BusiestDay:  busiest,
		ClaudeShare: share,
		CacheSaved:  cacheSavings,
	}
}

// WrappedData is the display-ready form of the Wrapped card: pre-formatted strings
// plus bar fractions relative to the top model. Port of BurntCore/WrappedData.swift.
type WrappedData struct {
	Title          string
	HeadlineCost   string
	HeadlineTokens string
	TopModelName   string
	ModelBars      []WrappedBar
	BusiestDay     string
	BusiestDayCost string
	ClaudeShare    float64 // 0...1
	CacheSaved     string
}

// WrappedBar is one model bar, sized as a fraction of the top model's cost.
type WrappedBar struct {
	Name     string
	Cost     float64
	Fraction float64 // 0...1 of the top model's cost
}

// NewWrappedData formats a Wrapped card. models may be in any order.
func NewWrappedData(title string, totalCost float64, totalTokens int64,
	models []WrappedModel, busiestDay string, busiestDayCost float64,
	claudeShare, cacheSaved float64) WrappedData {
	sorted := make([]WrappedModel, len(models))
	copy(sorted, models)
	sort.SliceStable(sorted, func(i, j int) bool { return sorted[i].Cost > sorted[j].Cost })

	topName := "—"
	topCost := 0.0
	if len(sorted) > 0 {
		topName = sorted[0].Model
		topCost = sorted[0].Cost
	}
	// Floor the divisor so an all-zero (or empty) model list can't divide by zero.
	if topCost < 0.0001 {
		topCost = 0.0001
	}

	bars := make([]WrappedBar, 0, wrappedTopModels)
	for _, m := range sorted {
		if len(bars) == wrappedTopModels {
			break
		}
		bars = append(bars, WrappedBar{Name: m.Model, Cost: m.Cost, Fraction: m.Cost / topCost})
	}

	return WrappedData{
		Title:          title,
		HeadlineCost:   FormatCost(totalCost),
		HeadlineTokens: FormatTokens(totalTokens),
		TopModelName:   topName,
		ModelBars:      bars,
		BusiestDay:     busiestDay,
		BusiestDayCost: FormatCost(busiestDayCost),
		ClaudeShare:    claudeShare,
		CacheSaved:     FormatCost(cacheSaved),
	}
}
