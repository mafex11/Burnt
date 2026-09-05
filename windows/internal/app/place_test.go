package app

import "testing"

// A 1080p screen with the taskbar at the bottom.
var work = Rect{Left: 0, Top: 0, Right: 1920, Bottom: 1032}

func TestPlacePopoverHangsOffTheCursor(t *testing.T) {
	// Cursor well inside the work area: no clamping, pure "bottom-right at cursor".
	x, y := PlacePopover(1700, 900, work, 320, 480, 12, 8)
	if x != 1700-320 {
		t.Errorf("x = %d, want the window's right edge at the cursor", x)
	}
	if y != 900-12-480 {
		t.Errorf("y = %d, want the window above the cursor", y)
	}
}

func TestPlacePopoverClampsInsideWorkArea(t *testing.T) {
	// Cursor at the very bottom-right: the window must not cross the taskbar.
	x, y := PlacePopover(1919, 1079, work, 320, 480, 12, 8)
	if x+320 > work.Right-8 {
		t.Errorf("x = %d overflows the right edge", x)
	}
	if y+480 > work.Bottom-8 {
		t.Errorf("y = %d overlaps the taskbar", y)
	}

	// Cursor top-left (taskbar moved to the top of the screen).
	topWork := Rect{Left: 0, Top: 48, Right: 1920, Bottom: 1080}
	x, y = PlacePopover(4, 40, topWork, 320, 480, 12, 8)
	if x < topWork.Left {
		t.Errorf("x = %d is off screen", x)
	}
	if y < topWork.Top+8 {
		t.Errorf("y = %d is above the work area", y)
	}
}

func TestPlacePopoverOversizedWindowPinsTopLeft(t *testing.T) {
	tiny := Rect{Left: 0, Top: 0, Right: 200, Bottom: 300}
	x, y := PlacePopover(190, 290, tiny, 320, 480, 12, 8)
	if x != tiny.Left || y != tiny.Top {
		t.Errorf("got (%d,%d), want the top-left corner", x, y)
	}
}

func TestPlacePopoverDropsGapWhenTight(t *testing.T) {
	// Work area exactly as tall as the window: the gap would push it off screen.
	tight := Rect{Left: 0, Top: 0, Right: 1920, Bottom: 480}
	_, y := PlacePopover(1000, 470, tight, 320, 480, 12, 0)
	if y != 0 {
		t.Errorf("y = %d, want 0", y)
	}
}

func TestPlacePopoverSecondMonitorOffsets(t *testing.T) {
	// A monitor to the left of the primary one has negative coordinates.
	left := Rect{Left: -1920, Top: 0, Right: 0, Bottom: 1032}
	x, y := PlacePopover(-100, 1030, left, 320, 480, 12, 8)
	if x < left.Left || x+320 > left.Right {
		t.Errorf("x = %d escaped the monitor", x)
	}
	if y+480 > left.Bottom {
		t.Errorf("y = %d escaped the monitor", y)
	}
}

func TestScaleDips(t *testing.T) {
	cases := []struct {
		dips int32
		dpi  uint32
		want int32
	}{
		{320, 96, 320},
		{320, 144, 480}, // 150 %
		{320, 192, 640}, // 200 %
		{320, 0, 320},   // GetDpiForWindow failed
	}
	for _, tc := range cases {
		if got := ScaleDips(tc.dips, tc.dpi); got != tc.want {
			t.Errorf("ScaleDips(%d, %d) = %d, want %d", tc.dips, tc.dpi, got, tc.want)
		}
	}
}

func TestClampHeightDips(t *testing.T) {
	cases := []struct {
		in   float64
		want int32
	}{{0, 1}, {-5, 1}, {575, 575}, {575.4, 575}, {575.6, 576}, {2000, PopoverMaxDips}}
	for _, tc := range cases {
		if got := ClampHeightDips(tc.in); got != tc.want {
			t.Errorf("ClampHeightDips(%v) = %d, want %d", tc.in, got, tc.want)
		}
	}
}
