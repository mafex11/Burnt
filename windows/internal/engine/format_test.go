package engine

import "testing"

func TestFormatTokens(t *testing.T) {
	cases := map[int64]string{
		0:              "0",
		999:            "999",
		1_000:          "1.0K",
		340_000:        "340K",
		1_234_567:      "1.2M",
		12_000_000_000: "12.0B",
		99_999:         "100.0K", // 99.999 rounds up in the one-decimal branch
		100_000:        "100K",
	}
	for n, want := range cases {
		if got := FormatTokens(n); got != want {
			t.Errorf("FormatTokens(%d) = %q, want %q", n, got, want)
		}
	}
}

func TestFormatCost(t *testing.T) {
	cases := map[float64]string{
		4.2:       "$4.20",
		0:         "$0.00",
		7468.3:    "$7,468",
		999.994:   "$999.99",
		1000:      "$1,000",
		1234567.8: "$1,234,568",
	}
	for c, want := range cases {
		if got := FormatCost(c); got != want {
			t.Errorf("FormatCost(%v) = %q, want %q", c, got, want)
		}
	}
}

func TestFormatPercent(t *testing.T) {
	cases := map[float64]string{0.12: "12%", -0.5: "50%", 1.24: "124%", 0: "0%"}
	for f, want := range cases {
		if got := FormatPercent(f); got != want {
			t.Errorf("FormatPercent(%v) = %q, want %q", f, got, want)
		}
	}
}

func TestFormatShortDate(t *testing.T) {
	cases := map[string]string{
		"2026-06-08": "Jun 8",
		"2026-01-30": "Jan 30",
		"2026-12-01": "Dec 1",
		"garbage":    "garbage",
		"2026-13-01": "2026-13-01",
	}
	for iso, want := range cases {
		if got := FormatShortDate(iso); got != want {
			t.Errorf("FormatShortDate(%q) = %q, want %q", iso, got, want)
		}
	}
}
