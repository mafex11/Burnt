package engine

import (
	"flag"
	"os"
	"testing"
)

// updateGolden re-records testdata/summary-golden.json instead of comparing.
var updateGolden = flag.Bool("update-golden", false, "rewrite the golden summary fixture")

func TestMain(m *testing.M) {
	flag.Parse()
	os.Exit(m.Run())
}
