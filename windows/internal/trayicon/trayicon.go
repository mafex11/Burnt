// Package trayicon builds the Windows tray icon.
//
// Windows tray icons carry no text label, so the dollar figure is drawn *into*
// the icon and the whole ICO is rebuilt on every refresh. Each ICO holds
// uncompressed 32-bpp BGRA frames at 16, 20, 24, 32 and 48 px, rendered natively
// at that size (never upscaled) so the shell picks a crisp one for the current
// DPI.
//
// The package is portable: it only touches golang.org/x/image, which keeps it
// testable on macOS.
package trayicon

import (
	"bytes"
	"embed"
	"encoding/binary"
	"fmt"
	"image"
	"image/color"
	"image/draw"
	"image/png"
	"math"
	"sync"

	xdraw "golang.org/x/image/draw"
	"golang.org/x/image/font"
	"golang.org/x/image/font/gofont/gobold"
	"golang.org/x/image/font/opentype"
	"golang.org/x/image/math/f64"
	"golang.org/x/image/math/fixed"
)

// flameFS holds the app icon (a rounded orange gradient square with a white
// flame) used for icon-only mode.
//
//go:embed flame.png
var flameFS embed.FS

// iconSizes are the frame sizes every ICO contains, smallest first.
var iconSizes = []int{16, 20, 24, 32, 48}

// PlaceholderText is shown when there is no usage data yet.
const PlaceholderText = "—"

// Text colours: near-black on a light taskbar, white on a dark one.
var (
	lightThemeText = color.NRGBA{R: 0x1a, G: 0x1a, B: 0x1a, A: 0xff}
	darkThemeText  = color.NRGBA{R: 0xff, G: 0xff, B: 0xff, A: 0xff}
)

// Sizes returns the frame sizes present in every ICO this package produces.
func Sizes() []int {
	out := make([]int, len(iconSizes))
	copy(out, iconSizes)
	return out
}

// TextColor is the colour RenderText draws with for the given theme.
func TextColor(light bool) color.NRGBA {
	if light {
		return lightThemeText
	}
	return darkThemeText
}

// RenderText returns a multi-size ICO with text drawn in bold, filling the
// width of each frame and centred vertically on its ink, on a transparent
// background. Pass light=true for a light taskbar.
func RenderText(text string, light bool) ([]byte, error) {
	if text == "" {
		text = PlaceholderText
	}
	frames := make([][]byte, 0, len(iconSizes))
	for _, size := range iconSizes {
		img, err := renderTextImage(text, size, TextColor(light))
		if err != nil {
			return nil, err
		}
		encoded, err := encodeDIB(img)
		if err != nil {
			return nil, err
		}
		frames = append(frames, encoded)
	}
	return buildICO(iconSizes, frames)
}

// Placeholder is the em dash icon shown before the first successful refresh.
// It is rendered once and cached; the inputs are constant so it cannot fail in
// practice, and a blank icon is preferable to a panic inside the tray loop.
func Placeholder() []byte {
	return placeholder()
}

var placeholder = sync.OnceValue(func() []byte {
	data, err := RenderText(PlaceholderText, false)
	if err != nil {
		blank, blankErr := blankICO()
		if blankErr != nil {
			return nil
		}
		return blank
	}
	return data
})

// Flame returns the app icon downscaled into a multi-size ICO, for icon-only
// mode. The artwork carries its own colours and reads on either taskbar theme,
// so light currently selects no variant; it exists so callers can pass the
// theme they already know without special-casing this mode.
func Flame(light bool) ([]byte, error) {
	src, err := flameSource()
	if err != nil {
		return nil, err
	}
	frames := make([][]byte, 0, len(iconSizes))
	for _, size := range iconSizes {
		img := image.NewNRGBA(image.Rect(0, 0, size, size))
		xdraw.CatmullRom.Scale(img, img.Bounds(), src, src.Bounds(), xdraw.Over, nil)
		encoded, err := encodeDIB(img)
		if err != nil {
			return nil, err
		}
		frames = append(frames, encoded)
	}
	return buildICO(iconSizes, frames)
}

// FlameFrames returns n ICOs forming a loop of gently warped flames (±4 % scale
// and ±3° of sway) for the animateFlame option. Frame 0 is upright, and the
// cycle is seamless so it can be played on repeat.
func FlameFrames(light bool, n int) ([][]byte, error) {
	if n <= 0 {
		return nil, fmt.Errorf("trayicon: frame count must be positive, got %d", n)
	}
	if n == 1 {
		one, err := Flame(light)
		if err != nil {
			return nil, err
		}
		return [][]byte{one}, nil
	}

	src, err := flameSource()
	if err != nil {
		return nil, err
	}

	out := make([][]byte, 0, n)
	for i := range n {
		phase := 2 * math.Pi * float64(i) / float64(n)
		// The lean and the breathing run a quarter-cycle apart, so the flame
		// appears to sway instead of merely pulsing, and no two frames repeat.
		angle := 3 * math.Pi / 180 * math.Sin(phase)
		scale := 1 + 0.04*math.Cos(phase)

		frames := make([][]byte, 0, len(iconSizes))
		for _, size := range iconSizes {
			img := warpFlame(src, size, scale, angle)
			encoded, err := encodeDIB(img)
			if err != nil {
				return nil, err
			}
			frames = append(frames, encoded)
		}
		ico, err := buildICO(iconSizes, frames)
		if err != nil {
			return nil, err
		}
		out = append(out, ico)
	}
	return out, nil
}

// warpFlame scales src into a size×size frame, magnified by scale and rotated
// by angle radians about the centre.
func warpFlame(src image.Image, size int, scale, angle float64) *image.NRGBA {
	dst := image.NewNRGBA(image.Rect(0, 0, size, size))
	sb := src.Bounds()
	// Shrink slightly so a rotated corner cannot clip against the frame edge.
	k := scale * 0.96 * float64(size) / float64(sb.Dx())
	sin, cos := math.Sin(angle), math.Cos(angle)
	scx, scy := float64(sb.Min.X)+float64(sb.Dx())/2, float64(sb.Min.Y)+float64(sb.Dy())/2
	dcx, dcy := float64(size)/2, float64(size)/2

	a00, a01 := k*cos, -k*sin
	a10, a11 := k*sin, k*cos
	s2d := f64.Aff3{
		a00, a01, dcx - (a00*scx + a01*scy),
		a10, a11, dcy - (a10*scx + a11*scy),
	}
	xdraw.CatmullRom.Transform(dst, s2d, src, sb, xdraw.Over, nil)
	return dst
}

// flameSource decodes the embedded PNG once and reuses it.
var flameSource = sync.OnceValues(func() (image.Image, error) {
	data, err := flameFS.ReadFile("flame.png")
	if err != nil {
		return nil, fmt.Errorf("trayicon: read embedded flame: %w", err)
	}
	img, err := png.Decode(bytes.NewReader(data))
	if err != nil {
		return nil, fmt.Errorf("trayicon: decode embedded flame: %w", err)
	}
	return img, nil
})

// boldFont parses the embedded Go Bold face once.
var boldFont = sync.OnceValues(func() (*opentype.Font, error) {
	f, err := opentype.Parse(gobold.TTF)
	if err != nil {
		return nil, fmt.Errorf("trayicon: parse Go Bold: %w", err)
	}
	return f, nil
})

// renderTextImage draws text as large as will fit in a size×size transparent
// frame, centred on its ink both horizontally and vertically.
func renderTextImage(text string, size int, c color.NRGBA) (*image.NRGBA, error) {
	fnt, err := boldFont()
	if err != nil {
		return nil, err
	}

	// One pixel of breathing room at 16 px, proportionally more when larger.
	pad := math.Max(1, math.Round(float64(size)*0.06))
	availW := float64(size) - 2*pad
	availH := float64(size) - 2*math.Max(1, pad-1)

	face, ink, err := fitFace(fnt, text, size, availW, availH)
	if err != nil {
		return nil, err
	}
	defer face.Close()

	img := image.NewNRGBA(image.Rect(0, 0, size, size))
	inkW := float64(ink.Max.X-ink.Min.X) / 64
	inkH := float64(ink.Max.Y-ink.Min.Y) / 64
	originX := (float64(size)-inkW)/2 - float64(ink.Min.X)/64
	baselineY := (float64(size)-inkH)/2 - float64(ink.Min.Y)/64

	d := font.Drawer{
		Dst:  img,
		Src:  image.NewUniform(c),
		Face: face,
		// Whole-pixel origin: hinted stems and the em dash's single-pixel bar stay
		// on the pixel grid instead of smearing across two rows.
		Dot: fixed.P(int(math.Round(originX)), int(math.Round(baselineY))),
	}
	d.DrawString(text)
	return img, nil
}

// fitFace finds the largest whole-point face whose ink fits inside availW ×
// availH, returning it along with the measured ink bounds. The caller closes
// the face.
func fitFace(fnt *opentype.Font, text string, size int, availW, availH float64) (font.Face, fixed.Rectangle26_6, error) {
	// A single glyph can be far taller than wide, so start well above the frame
	// size and let the height check pull it back.
	var last font.Face
	var lastInk fixed.Rectangle26_6
	for pt := float64(size) * 2; pt >= 4; pt -= 0.5 {
		face, err := opentype.NewFace(fnt, &opentype.FaceOptions{
			Size:    pt,
			DPI:     72,
			Hinting: font.HintingFull,
		})
		if err != nil {
			return nil, fixed.Rectangle26_6{}, fmt.Errorf("trayicon: build face at %gpt: %w", pt, err)
		}
		ink, _ := font.BoundString(face, text)
		w := float64(ink.Max.X-ink.Min.X) / 64
		h := float64(ink.Max.Y-ink.Min.Y) / 64
		if w <= availW && h <= availH {
			return face, ink, nil
		}
		if last != nil {
			last.Close()
		}
		last, lastInk = face, ink
	}
	// Nothing fit (a very long string in a 16 px box): use the smallest face and
	// let it be clipped rather than failing the refresh.
	if last != nil {
		return last, lastInk, nil
	}
	return nil, fixed.Rectangle26_6{}, fmt.Errorf("trayicon: no usable face for %q at %dpx", text, size)
}

// DIB header and mask layout constants.
const (
	dibHeaderSize = 40 // BITMAPINFOHEADER
	dibBitCount   = 32
	biRGB         = 0 // uncompressed
)

// encodeDIB encodes img as an icon image: a BITMAPINFOHEADER, bottom-up 32-bpp
// BGRA pixels with straight (non-premultiplied) alpha, then a 1-bpp AND mask.
//
// This is the format LoadImage(LR_LOADFROMFILE) has always understood; PNG
// payloads below 256 px are not decoded reliably by that path, which is how
// fyne.io/systray installs the icon. The AND mask is all zero bits (meaning
// "opaque") because the alpha channel already carries transparency, but it must
// still be present and 4-byte aligned or the shell reads garbage.
func encodeDIB(img image.Image) ([]byte, error) {
	b := img.Bounds()
	w, h := b.Dx(), b.Dy()
	if w <= 0 || h <= 0 {
		return nil, fmt.Errorf("trayicon: cannot encode a %dx%d icon", w, h)
	}

	xorStride := w * 4                // 32 bpp rows are inherently aligned
	maskStride := ((w + 31) / 32) * 4 // 1 bpp rows padded to 4 bytes
	xorBytes := xorStride * h
	maskBytes := maskStride * h

	buf := bytes.NewBuffer(make([]byte, 0, dibHeaderSize+xorBytes+maskBytes))
	putU16 := func(v uint16) { binary.Write(buf, binary.LittleEndian, v) }
	putU32 := func(v uint32) { binary.Write(buf, binary.LittleEndian, v) }
	putI32 := func(v int32) { binary.Write(buf, binary.LittleEndian, v) }

	putU32(dibHeaderSize)
	putI32(int32(w))
	putI32(int32(2 * h)) // XOR image plus AND mask, per the ICO convention
	putU16(1)            // biPlanes
	putU16(dibBitCount)
	putU32(biRGB)
	putU32(uint32(xorBytes + maskBytes)) // biSizeImage
	putI32(0)                            // biXPelsPerMeter
	putI32(0)                            // biYPelsPerMeter
	putU32(0)                            // biClrUsed
	putU32(0)                            // biClrImportant

	// XOR pixels, bottom row first.
	row := make([]byte, xorStride)
	for y := b.Max.Y - 1; y >= b.Min.Y; y-- {
		for x := 0; x < w; x++ {
			c := color.NRGBAModel.Convert(img.At(b.Min.X+x, y)).(color.NRGBA)
			i := x * 4
			row[i+0] = c.B
			row[i+1] = c.G
			row[i+2] = c.R
			row[i+3] = c.A
		}
		buf.Write(row)
	}
	// AND mask: every bit clear.
	buf.Write(make([]byte, maskBytes))

	return buf.Bytes(), nil
}

// blankICO is the fallback when even placeholder rendering fails.
func blankICO() ([]byte, error) {
	frames := make([][]byte, 0, len(iconSizes))
	for _, size := range iconSizes {
		img := image.NewNRGBA(image.Rect(0, 0, size, size))
		draw.Draw(img, img.Bounds(), image.Transparent, image.Point{}, draw.Src)
		encoded, err := encodeDIB(img)
		if err != nil {
			return nil, err
		}
		frames = append(frames, encoded)
	}
	return buildICO(iconSizes, frames)
}

// ICO container constants.
const (
	icoDirSize   = 6  // ICONDIR: reserved, type, count
	icoEntrySize = 16 // ICONDIRENTRY
	icoTypeIcon  = 1
	icoBitCount  = 32
)

// buildICO writes an ICO whose entries are the given DIB payloads.
//
// Layout: ICONDIR, then one ICONDIRENTRY per frame, then the payloads. Width and
// height are single bytes, so 256 px is encoded as 0; anything larger has no
// representation. See
// https://learn.microsoft.com/en-us/previous-versions/ms997538(v=msdn.10).
func buildICO(sizes []int, frames [][]byte) ([]byte, error) {
	if len(sizes) != len(frames) {
		return nil, fmt.Errorf("trayicon: %d sizes but %d frames", len(sizes), len(frames))
	}
	if len(frames) == 0 {
		return nil, fmt.Errorf("trayicon: ICO needs at least one frame")
	}

	var buf bytes.Buffer
	writeU16 := func(v uint16) { binary.Write(&buf, binary.LittleEndian, v) }
	writeU32 := func(v uint32) { binary.Write(&buf, binary.LittleEndian, v) }

	writeU16(0) // reserved
	writeU16(icoTypeIcon)
	writeU16(uint16(len(frames)))

	offset := icoDirSize + icoEntrySize*len(frames)
	for i, size := range sizes {
		if size <= 0 || size > 256 {
			return nil, fmt.Errorf("trayicon: unsupported frame size %d", size)
		}
		dim := byte(size)
		if size == 256 {
			dim = 0
		}
		buf.WriteByte(dim) // width
		buf.WriteByte(dim) // height
		buf.WriteByte(0)   // palette size; 0 for truecolour
		buf.WriteByte(0)   // reserved
		writeU16(1)        // colour planes
		writeU16(icoBitCount)
		writeU32(uint32(len(frames[i])))
		writeU32(uint32(offset))
		offset += len(frames[i])
	}
	for _, f := range frames {
		buf.Write(f)
	}
	return buf.Bytes(), nil
}
