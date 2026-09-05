package trayicon

import (
	"math"
	"strconv"
)

// costUnits and tokenUnits mirror the macOS menu bar: money uses a lowercase
// thousands suffix ("$1.2k"), token counts an uppercase one ("340K").
var (
	costUnits  = []string{"", "k", "M", "B"}
	tokenUnits = []string{"", "K", "M", "B"}
)

// CompactCost renders a dollar amount short enough to fit inside a 16 px tray
// icon: at most four characters after the "$".
//
//	3.94      -> "$3.9"   (under $10 keeps one decimal)
//	123.4     -> "$123"   (under $1000 rounds to whole dollars)
//	1234.0    -> "$1.2k"
//	12345.0   -> "$12k"
//	1234567.0 -> "$1.2M"
//
// Full precision lives in the tooltip and menu, not here.
func CompactCost(cost float64) string {
	return "$" + compact(cost, costUnits, true)
}

// CompactTokens renders a token count in at most four characters:
// 340_000 -> "340K", 1_234_567 -> "1.2M", 12_000_000 -> "12M".
func CompactTokens(n int64) string {
	return compact(float64(n), tokenUnits, false)
}

// compact scales v down through units until it fits in four characters, keeping
// one decimal only while the mantissa is a single digit. decimalAtBase controls
// whether an unscaled value may keep its decimal: dollars want "$3.9", but a
// raw token count is a whole number ("7", not "7.0").
func compact(v float64, units []string, decimalAtBase bool) string {
	if math.IsNaN(v) || v <= 0 {
		return "0"
	}
	if math.IsInf(v, 1) {
		v = math.MaxFloat64
	}

	i := 0
	// 999.5 and up would render as four digits, so promote it to the next unit.
	for v >= 999.5 && i < len(units)-1 {
		v /= 1000
		i++
	}
	// Anything past the largest unit is clamped rather than overflowing the icon.
	if v >= 999.5 {
		v = 999
	}

	// 9.95 rounds to "10.0", which is one character too many, so switch to a
	// whole number at that point.
	if v < 9.95 && (i > 0 || decimalAtBase) {
		return strconv.FormatFloat(math.Round(v*10)/10, 'f', 1, 64) + units[i]
	}
	return strconv.FormatFloat(math.Round(v), 'f', 0, 64) + units[i]
}
