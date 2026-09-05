package engine

import (
	"fmt"
	"math"
	"strconv"
	"strings"
)

// FormatCost renders a dollar amount: "$4.20", and at $1000+ drops the cents and
// groups thousands ("$7,468"). Port of BurntCore/Formatters.cost.
func FormatCost(c float64) string {
	if c >= 1000 {
		return "$" + groupThousands(strconv.FormatInt(int64(math.Round(c)), 10))
	}
	return fmt.Sprintf("$%.2f", c)
}

// FormatTokens renders a compact token count: 1234567 -> "1.2M", 340000 -> "340K".
// Port of BurntCore/Formatters.tokens.
func FormatTokens(n int64) string {
	v := float64(n)
	switch {
	case v >= 1_000_000_000:
		return trimUnit(v/1_000_000_000) + "B"
	case v >= 1_000_000:
		return trimUnit(v/1_000_000) + "M"
	case v >= 1_000:
		return trimUnit(v/1_000) + "K"
	default:
		return strconv.FormatInt(n, 10)
	}
}

// FormatPercent renders a fraction's magnitude as a whole percent:
// 0.12 -> "12%", -0.5 -> "50%", 1.24 -> "124%".
func FormatPercent(fraction float64) string {
	return strconv.Itoa(int(math.Round(math.Abs(fraction)*100))) + "%"
}

var shortMonths = [12]string{"Jan", "Feb", "Mar", "Apr", "May", "Jun",
	"Jul", "Aug", "Sep", "Oct", "Nov", "Dec"}

// FormatShortDate turns "2026-06-08" into "Jun 8", passing through anything it
// can't parse. Port of AppModel.prettyDate.
func FormatShortDate(iso string) string {
	parts := strings.Split(iso, "-")
	if len(parts) != 3 {
		return iso
	}
	month, err := strconv.Atoi(parts[1])
	if err != nil || month < 1 || month > 12 {
		return iso
	}
	day, err := strconv.Atoi(parts[2])
	if err != nil {
		return iso
	}
	return fmt.Sprintf("%s %d", shortMonths[month-1], day)
}

// trimUnit renders a scaled magnitude: 340.0 -> "340", 1.23 -> "1.2".
func trimUnit(x float64) string {
	if x >= 100 {
		return strconv.Itoa(int(math.Round(x)))
	}
	return fmt.Sprintf("%.1f", x)
}

// groupThousands inserts commas into a plain integer string.
func groupThousands(s string) string {
	neg := strings.HasPrefix(s, "-")
	digits := strings.TrimPrefix(s, "-")
	var b strings.Builder
	if neg {
		b.WriteByte('-')
	}
	for i, r := range digits {
		if i > 0 && (len(digits)-i)%3 == 0 {
			b.WriteByte(',')
		}
		b.WriteRune(r)
	}
	return b.String()
}
