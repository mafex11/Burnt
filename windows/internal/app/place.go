package app

// Rect is a Win32 RECT (physical pixels, screen coordinates).
type Rect struct {
	Left, Top, Right, Bottom int32
}

// Width and Height are the obvious things; a work area is never inverted.
func (r Rect) Width() int32  { return r.Right - r.Left }
func (r Rect) Height() int32 { return r.Bottom - r.Top }

// Layout constants, in device-independent pixels (dips).
const (
	PopoverWidthDips = 320 // ui/styles.css is built for exactly this
	PopoverMaxDips   = 720 // app.js pre-clamps to this; we enforce it too
	PopoverInitDips  = 480 // before the first burnt_resize arrives
	CursorGapDips    = 12  // breathing room between the cursor and the popover
	EdgeMarginDips   = 8   // never sit flush against the work area edge
)

// ScaleDips converts dips to physical pixels for a given window DPI.
func ScaleDips(dips int32, dpi uint32) int32 {
	if dpi == 0 {
		dpi = 96
	}
	px := int32(int64(dips) * int64(dpi) / 96)
	if px < 1 {
		px = 1
	}
	return px
}

// ClampHeightDips bounds a height reported by burnt_resize.
func ClampHeightDips(h float64) int32 {
	switch {
	case h < 1:
		return 1
	case h > PopoverMaxDips:
		return PopoverMaxDips
	default:
		return int32(h + 0.5)
	}
}

// PlacePopover positions a w×h window (all arguments in physical pixels) so its
// bottom-right corner sits just above and left of the cursor — hanging off the tray
// icon the user clicked — then clamps the whole window inside the work area so it
// never slides under the taskbar or off a monitor.
//
// gap is the vertical offset from the cursor; margin keeps the window off the work
// area's edges. Both are dropped when the window only just fits, and a window bigger
// than the work area is pinned to its top-left corner so the header stays visible.
func PlacePopover(cursorX, cursorY int32, work Rect, w, h, gap, margin int32) (x, y int32) {
	if work.Height() <= h+gap {
		gap = 0
	}
	x = clampSpan(cursorX-w, w, work.Left, work.Right, margin)
	y = clampSpan(cursorY-gap-h, h, work.Top, work.Bottom, margin)
	return x, y
}

// clampSpan slides a size-long span to sit inside [lo, hi), respecting margin when
// there is room for it and giving up (pinning to lo) when the span simply doesn't fit.
func clampSpan(pos, size, lo, hi, margin int32) int32 {
	if size+2*margin > hi-lo {
		margin = 0
	}
	if size > hi-lo {
		return lo
	}
	if pos+size > hi-margin {
		pos = hi - margin - size
	}
	if pos < lo+margin {
		pos = lo + margin
	}
	return pos
}
