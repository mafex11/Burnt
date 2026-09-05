package trayicon

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCompactCost(t *testing.T) {
	tests := []struct {
		in   float64
		want string
	}{
		{0, "$0"},
		{-5, "$0"},
		{math.NaN(), "$0"},
		{0.04, "$0.0"},
		{0.12, "$0.1"},
		{3.94, "$3.9"},
		{3.95, "$4.0"},
		{9.94, "$9.9"},
		{9.95, "$10"},
		{9.99, "$10"},
		{12.34, "$12"},
		{12.6, "$13"},
		{123.4, "$123"},
		{999.4, "$999"},
		{999.6, "$1.0k"},
		{1000, "$1.0k"},
		{1234, "$1.2k"},
		{1250, "$1.3k"},
		{9949, "$9.9k"},
		{9999, "$10k"},
		{12345, "$12k"},
		{99999, "$100k"},
		{999499, "$999k"},
		{999600, "$1.0M"},
		{1234567, "$1.2M"},
		{12345678, "$12M"},
		{1.5e9, "$1.5B"},
		{math.Inf(1), "$999B"},
	}
	for _, tt := range tests {
		if got := CompactCost(tt.in); got != tt.want {
			t.Errorf("CompactCost(%v) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCompactCostFitsFourCharacters(t *testing.T) {
	// The "$" is drawn as a fifth glyph; the value itself must stay within four.
	for _, v := range []float64{0, 0.01, 9.99, 10, 99.9, 999, 1000, 9999, 1e4, 1e5,
		999999, 1e6, 1e7, 1e8, 1e9, 1e12, math.Inf(1)} {
		got := CompactCost(v)
		if !strings.HasPrefix(got, "$") {
			t.Errorf("CompactCost(%v) = %q, missing $", v, got)
		}
		if n := len([]rune(strings.TrimPrefix(got, "$"))); n > 4 {
			t.Errorf("CompactCost(%v) = %q, %d chars after $ (max 4)", v, got, n)
		}
	}
}

func TestCompactTokens(t *testing.T) {
	tests := []struct {
		in   int64
		want string
	}{
		{0, "0"},
		{-1, "0"},
		{7, "7"},
		{999, "999"},
		{1000, "1.0K"},
		{1234, "1.2K"},
		{9990, "10K"},
		{12345, "12K"},
		{340_000, "340K"},
		{999_400, "999K"},
		{1_000_000, "1.0M"},
		{1_234_567, "1.2M"},
		{12_000_000, "12M"},
		{340_000_000, "340M"},
		{1_234_000_000, "1.2B"},
	}
	for _, tt := range tests {
		if got := CompactTokens(tt.in); got != tt.want {
			t.Errorf("CompactTokens(%d) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestCompactTokensFitsFourCharacters(t *testing.T) {
	for _, n := range []int64{0, 1, 999, 1000, 9999, 340_000, 1_234_567,
		12_000_000, 999_999_999, 1 << 62} {
		if got := CompactTokens(n); len([]rune(got)) > 4 {
			t.Errorf("CompactTokens(%d) = %q, %d chars (max 4)", n, got, len(got))
		}
	}
}

// icoFrame is one decoded ICONDIRENTRY plus its payload.
type icoFrame struct {
	width, height int
	bitCount      uint16
	payload       []byte
}

// parseICO validates the container and returns its frames.
func parseICO(t *testing.T, data []byte) []icoFrame {
	t.Helper()
	if len(data) < icoDirSize {
		t.Fatalf("ICO too short: %d bytes", len(data))
	}
	if got := binary.LittleEndian.Uint16(data[0:2]); got != 0 {
		t.Errorf("reserved = %d, want 0", got)
	}
	if got := binary.LittleEndian.Uint16(data[2:4]); got != icoTypeIcon {
		t.Errorf("type = %d, want 1 (icon)", got)
	}
	count := int(binary.LittleEndian.Uint16(data[4:6]))
	if count == 0 {
		t.Fatal("ICO has no entries")
	}
	if len(data) < icoDirSize+icoEntrySize*count {
		t.Fatalf("ICO truncated: %d bytes for %d entries", len(data), count)
	}

	frames := make([]icoFrame, 0, count)
	for i := range count {
		e := data[icoDirSize+icoEntrySize*i:][:icoEntrySize]
		w, h := int(e[0]), int(e[1])
		if w == 0 {
			w = 256
		}
		if h == 0 {
			h = 256
		}
		if e[2] != 0 || e[3] != 0 {
			t.Errorf("entry %d: palette/reserved = %d/%d, want 0/0", i, e[2], e[3])
		}
		if planes := binary.LittleEndian.Uint16(e[4:6]); planes != 1 {
			t.Errorf("entry %d: planes = %d, want 1", i, planes)
		}
		size := binary.LittleEndian.Uint32(e[8:12])
		off := binary.LittleEndian.Uint32(e[12:16])
		if int(off)+int(size) > len(data) {
			t.Fatalf("entry %d: payload %d..%d outside %d bytes", i, off, int(off)+int(size), len(data))
		}
		frames = append(frames, icoFrame{
			width:    w,
			height:   h,
			bitCount: binary.LittleEndian.Uint16(e[6:8]),
			payload:  data[off : int(off)+int(size)],
		})
	}
	return frames
}

// decodeDIB validates one icon payload's BITMAPINFOHEADER against the size the
// directory declared and returns its XOR pixels as a top-down image.
func decodeDIB(t *testing.T, payload []byte, want int) image.Image {
	t.Helper()
	if len(payload) < dibHeaderSize {
		t.Fatalf("payload is %d bytes, shorter than a BITMAPINFOHEADER", len(payload))
	}
	u32 := func(off int) uint32 { return binary.LittleEndian.Uint32(payload[off : off+4]) }
	i32 := func(off int) int32 { return int32(u32(off)) }
	u16 := func(off int) uint16 { return binary.LittleEndian.Uint16(payload[off : off+2]) }

	if got := u32(0); got != dibHeaderSize {
		t.Errorf("biSize = %d, want %d", got, dibHeaderSize)
	}
	w, doubleH := int(i32(4)), int(i32(8))
	if w != want {
		t.Errorf("biWidth = %d, want %d", w, want)
	}
	// ICO stores the XOR image and the AND mask stacked, so the header height is
	// twice the real one.
	if doubleH != 2*want {
		t.Errorf("biHeight = %d, want %d (2x%d)", doubleH, 2*want, want)
	}
	if got := u16(12); got != 1 {
		t.Errorf("biPlanes = %d, want 1", got)
	}
	if got := u16(14); got != dibBitCount {
		t.Errorf("biBitCount = %d, want 32", got)
	}
	if got := u32(16); got != biRGB {
		t.Errorf("biCompression = %d, want 0 (BI_RGB)", got)
	}
	if got := u32(32); got != 0 {
		t.Errorf("biClrUsed = %d, want 0", got)
	}

	h := doubleH / 2
	xorStride := w * 4
	maskStride := ((w + 31) / 32) * 4
	xorBytes, maskBytes := xorStride*h, maskStride*h
	if got := u32(20); int(got) != xorBytes+maskBytes {
		t.Errorf("biSizeImage = %d, want %d (XOR %d + AND %d)", got, xorBytes+maskBytes, xorBytes, maskBytes)
	}
	if len(payload) != dibHeaderSize+xorBytes+maskBytes {
		t.Fatalf("payload is %d bytes, want %d", len(payload), dibHeaderSize+xorBytes+maskBytes)
	}
	for i, b := range payload[dibHeaderSize+xorBytes:] {
		if b != 0 {
			t.Errorf("AND mask byte %d = %#x, want 0 (alpha carries transparency)", i, b)
			break
		}
	}

	// XOR rows are bottom-up BGRA with straight alpha.
	img := image.NewNRGBA(image.Rect(0, 0, w, h))
	for row := range h {
		src := payload[dibHeaderSize+row*xorStride:][:xorStride]
		y := h - 1 - row
		for x := range w {
			p := src[x*4:]
			img.SetNRGBA(x, y, color.NRGBA{R: p[2], G: p[1], B: p[0], A: p[3]})
		}
	}
	return img
}

// checkFrames asserts the ICO holds exactly the expected sizes as uncompressed
// DIB entries, and returns them decoded.
func checkFrames(t *testing.T, data []byte) []image.Image {
	t.Helper()
	frames := parseICO(t, data)
	want := Sizes()
	if len(frames) != len(want) {
		t.Fatalf("got %d frames, want %d", len(frames), len(want))
	}
	images := make([]image.Image, 0, len(frames))
	for i, f := range frames {
		if f.width != want[i] || f.height != want[i] {
			t.Errorf("frame %d: declared %dx%d, want %dx%d", i, f.width, f.height, want[i], want[i])
		}
		if f.bitCount != icoBitCount {
			t.Errorf("frame %d: bitCount = %d, want 32", i, f.bitCount)
		}
		if bytes.HasPrefix(f.payload, []byte("\x89PNG\r\n\x1a\n")) {
			t.Errorf("frame %d: payload is PNG; LoadImage needs an uncompressed DIB", i)
			continue
		}
		img := decodeDIB(t, f.payload, want[i])
		b := img.Bounds()
		if b.Dx() != f.width || b.Dy() != f.height {
			t.Errorf("frame %d: DIB is %dx%d, declared %dx%d", i, b.Dx(), b.Dy(), f.width, f.height)
		}
		images = append(images, img)
	}
	return images
}

func TestSizes(t *testing.T) {
	want := []int{16, 20, 24, 32, 48}
	got := Sizes()
	if len(got) != len(want) {
		t.Fatalf("Sizes() = %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("Sizes() = %v, want %v", got, want)
		}
	}
	got[0] = 99 // must be a copy
	if Sizes()[0] != 16 {
		t.Error("Sizes() leaks its backing array")
	}
}

func TestRenderTextFrames(t *testing.T) {
	for _, text := range []string{"$3.9", "$123", "$1.2k", "340K", PlaceholderText} {
		for _, light := range []bool{false, true} {
			data, err := RenderText(text, light)
			if err != nil {
				t.Fatalf("RenderText(%q, %v): %v", text, light, err)
			}
			images := checkFrames(t, data)
			for i, img := range images {
				opaque, wrong := countInk(img, TextColor(light))
				if opaque == 0 {
					t.Errorf("RenderText(%q, %v) frame %d has no ink", text, light, i)
				}
				if wrong > 0 {
					t.Errorf("RenderText(%q, %v) frame %d: %d ink pixels are the wrong colour",
						text, light, i, wrong)
				}
			}
			writePreview(t, images, fmt.Sprintf("text-%s-%s", safeName(text), themeName(light)))
		}
	}
}

func TestRenderTextFillsWidth(t *testing.T) {
	// Four characters should use most of the icon: at least 70 % of the width and
	// a third of the height, or the figure is unreadable on the taskbar.
	data, err := RenderText("$1.2k", false)
	if err != nil {
		t.Fatal(err)
	}
	for i, img := range checkFrames(t, data) {
		b := inkBounds(img)
		size := img.Bounds().Dx()
		if b.Empty() {
			t.Fatalf("frame %d is blank", i)
		}
		if float64(b.Dx()) < 0.7*float64(size) {
			t.Errorf("frame %d (%dpx): ink is %dpx wide, want >= %.0f", i, size, b.Dx(), 0.7*float64(size))
		}
		if float64(b.Dy()) < 0.3*float64(size) {
			t.Errorf("frame %d (%dpx): ink is %dpx tall, want >= %.0f", i, size, b.Dy(), 0.3*float64(size))
		}
		if b.Dx() > size || b.Dy() > size {
			t.Errorf("frame %d: ink %v overflows %dpx frame", i, b, size)
		}
	}
}

func TestRenderTextCentred(t *testing.T) {
	data, err := RenderText("$123", false)
	if err != nil {
		t.Fatal(err)
	}
	for i, img := range checkFrames(t, data) {
		size := img.Bounds().Dx()
		b := inkBounds(img)
		left, right := b.Min.X, size-b.Max.X
		top, bottom := b.Min.Y, size-b.Max.Y
		if math.Abs(float64(left-right)) > 2 {
			t.Errorf("frame %d (%dpx): horizontal margins %d/%d not centred", i, size, left, right)
		}
		if math.Abs(float64(top-bottom)) > 2 {
			t.Errorf("frame %d (%dpx): vertical margins %d/%d not centred", i, size, top, bottom)
		}
	}
}

func TestRenderTextEmptyUsesPlaceholder(t *testing.T) {
	empty, err := RenderText("", false)
	if err != nil {
		t.Fatal(err)
	}
	dash, err := RenderText(PlaceholderText, false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(empty, dash) {
		t.Error(`RenderText("") should render the placeholder`)
	}
}

func TestPlaceholder(t *testing.T) {
	data := Placeholder()
	if len(data) == 0 {
		t.Fatal("Placeholder() returned no data")
	}
	images := checkFrames(t, data)
	for i, img := range images {
		if opaque, _ := countInk(img, TextColor(false)); opaque == 0 {
			t.Errorf("placeholder frame %d has no ink", i)
		}
	}
	if !bytes.Equal(data, Placeholder()) {
		t.Error("Placeholder() should be cached and stable")
	}
	writePreview(t, images, "placeholder")
}

func TestFlame(t *testing.T) {
	for _, light := range []bool{false, true} {
		data, err := Flame(light)
		if err != nil {
			t.Fatalf("Flame(%v): %v", light, err)
		}
		images := checkFrames(t, data)
		for i, img := range images {
			if coverage(img) < 0.4 {
				t.Errorf("Flame(%v) frame %d covers only %.2f of the frame", light, i, coverage(img))
			}
		}
		writePreview(t, images, "flame-"+themeName(light))
	}
}

func TestFlameFrames(t *testing.T) {
	const n = 6
	frames, err := FlameFrames(false, n)
	if err != nil {
		t.Fatalf("FlameFrames: %v", err)
	}
	if len(frames) != n {
		t.Fatalf("got %d frames, want %d", len(frames), n)
	}
	for i, ico := range frames {
		images := checkFrames(t, ico)
		for j, img := range images {
			if coverage(img) < 0.3 {
				t.Errorf("frame %d size %d covers only %.2f", i, Sizes()[j], coverage(img))
			}
		}
		writePreview(t, images, fmt.Sprintf("flame-anim-%d", i))
	}
	for i := range frames {
		for j := i + 1; j < len(frames); j++ {
			if bytes.Equal(frames[i], frames[j]) {
				t.Errorf("frames %d and %d are identical; the animation would stall", i, j)
			}
		}
	}
}

func TestFlameFramesEdgeCases(t *testing.T) {
	if _, err := FlameFrames(false, 0); err == nil {
		t.Error("FlameFrames(0) should error")
	}
	if _, err := FlameFrames(false, -3); err == nil {
		t.Error("FlameFrames(-3) should error")
	}
	one, err := FlameFrames(false, 1)
	if err != nil {
		t.Fatal(err)
	}
	if len(one) != 1 {
		t.Fatalf("got %d frames, want 1", len(one))
	}
	still, err := Flame(false)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(one[0], still) {
		t.Error("a single frame should be the upright flame")
	}
}

func TestDIBPayloadLayout(t *testing.T) {
	// A 16 px entry is the classic 1128 bytes: 40 header + 1024 XOR + 64 AND.
	data, err := RenderText("$3.9", false)
	if err != nil {
		t.Fatal(err)
	}
	frames := parseICO(t, data)
	if got := len(frames[0].payload); got != 40+16*16*4+4*16 {
		t.Errorf("16px payload = %d bytes, want %d", got, 40+16*16*4+4*16)
	}
	// Offsets must be contiguous and start after the directory.
	off := icoDirSize + icoEntrySize*len(frames)
	total := off
	for i, f := range frames {
		if bytes.IndexByte(f.payload, 0) < 0 {
			t.Errorf("frame %d: payload looks empty", i)
		}
		total += len(f.payload)
	}
	if total != len(data) {
		t.Errorf("ICO is %d bytes, entries account for %d", len(data), total)
	}
}

func TestBuildICOErrors(t *testing.T) {
	if _, err := buildICO(nil, nil); err == nil {
		t.Error("empty ICO should error")
	}
	if _, err := buildICO([]int{16, 20}, [][]byte{{1}}); err == nil {
		t.Error("mismatched sizes/frames should error")
	}
	if _, err := buildICO([]int{257}, [][]byte{{1}}); err == nil {
		t.Error("size above 256 should error")
	}
	if _, err := buildICO([]int{0}, [][]byte{{1}}); err == nil {
		t.Error("size 0 should error")
	}
	// 256 px is encoded as a zero dimension byte.
	data, err := buildICO([]int{256}, [][]byte{{1, 2, 3}})
	if err != nil {
		t.Fatal(err)
	}
	if data[icoDirSize] != 0 || data[icoDirSize+1] != 0 {
		t.Errorf("256px should encode as 0/0, got %d/%d", data[icoDirSize], data[icoDirSize+1])
	}
}

// countInk returns how many mostly opaque pixels an image has, and how many of
// those are not the expected colour. Comparison happens on straight (not
// premultiplied) values, since antialiased pixels keep the text colour and vary
// only in alpha.
func countInk(img image.Image, want color.NRGBA) (opaque, wrong int) {
	b := img.Bounds()
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			c := color.NRGBAModel.Convert(img.At(x, y)).(color.NRGBA)
			if c.A < 0x80 {
				continue
			}
			opaque++
			// Round-tripping through premultiplied alpha shifts components a little.
			const tol = 12
			if absDiff(c.R, want.R) > tol || absDiff(c.G, want.G) > tol || absDiff(c.B, want.B) > tol {
				wrong++
			}
		}
	}
	return opaque, wrong
}

func absDiff(a, b uint8) uint8 {
	if a > b {
		return a - b
	}
	return b - a
}

// inkBounds is the tightest rectangle containing any non-transparent pixel.
func inkBounds(img image.Image) image.Rectangle {
	b := img.Bounds()
	out := image.Rectangle{}
	first := true
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a < 0x2000 {
				continue
			}
			p := image.Rect(x, y, x+1, y+1)
			if first {
				out, first = p, false
			} else {
				out = out.Union(p)
			}
		}
	}
	return out
}

// coverage is the fraction of pixels that are at least half opaque.
func coverage(img image.Image) float64 {
	b := img.Bounds()
	var n int
	for y := b.Min.Y; y < b.Max.Y; y++ {
		for x := b.Min.X; x < b.Max.X; x++ {
			if _, _, _, a := img.At(x, y).RGBA(); a >= 0x8000 {
				n++
			}
		}
	}
	return float64(n) / float64(b.Dx()*b.Dy())
}

func themeName(light bool) string {
	if light {
		return "light"
	}
	return "dark"
}

// safeName turns icon text into something a filesystem accepts.
func safeName(s string) string {
	r := strings.NewReplacer("$", "usd", ".", "-", "—", "dash", "/", "_")
	out := r.Replace(s)
	if out == "" {
		out = "empty"
	}
	return out
}

// writePreview dumps frames as PNGs when TRAYICON_PREVIEW_DIR is set, so the
// icons can be eyeballed after a test run.
func writePreview(t *testing.T, images []image.Image, name string) {
	t.Helper()
	dir := os.Getenv("TRAYICON_PREVIEW_DIR")
	if dir == "" {
		return
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		t.Fatalf("preview dir: %v", err)
	}
	for _, img := range images {
		size := img.Bounds().Dx()
		path := filepath.Join(dir, fmt.Sprintf("%s-%02d.png", name, size))
		f, err := os.Create(path)
		if err != nil {
			t.Fatalf("preview %s: %v", path, err)
		}
		if err := png.Encode(f, img); err != nil {
			f.Close()
			t.Fatalf("preview %s: %v", path, err)
		}
		if err := f.Close(); err != nil {
			t.Fatalf("preview %s: %v", path, err)
		}
	}
}
