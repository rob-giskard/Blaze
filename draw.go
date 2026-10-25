package main

import (
	"image"
	"image/color"
	"math"

	"github.com/hajimehoshi/ebiten/v2"
	"github.com/hajimehoshi/ebiten/v2/text/v2"
	"github.com/hajimehoshi/ebiten/v2/vector"
)

var (
	whiteImg = ebiten.NewImage(3, 3)
	whiteSub = whiteImg.SubImage(image.Rect(1, 1, 2, 2)).(*ebiten.Image)
)

func init() { whiteImg.Fill(color.White) }

func hex(s string) color.NRGBA {
	var v [3]uint8
	for i := 0; i < 3; i++ {
		v[i] = hexByte(s[1+i*2], s[2+i*2])
	}
	return color.NRGBA{v[0], v[1], v[2], 255}
}

func hexByte(a, b byte) uint8 { return hexNib(a)<<4 | hexNib(b) }
func hexNib(c byte) uint8 {
	switch {
	case c >= '0' && c <= '9':
		return c - '0'
	case c >= 'a' && c <= 'f':
		return c - 'a' + 10
	case c >= 'A' && c <= 'F':
		return c - 'A' + 10
	}
	return 0
}

func withAlpha(c color.NRGBA, a float64) color.NRGBA {
	if a < 0 {
		a = 0
	}
	if a > 1 {
		a = 1
	}
	c.A = uint8(float64(c.A) * a)
	return c
}

func lerpCol(a, b color.NRGBA, t float64) color.NRGBA {
	f := func(x, y uint8) uint8 { return uint8(float64(x) + (float64(y)-float64(x))*t) }
	return color.NRGBA{f(a.R, b.R), f(a.G, b.G), f(a.B, b.B), f(a.A, b.A)}
}

func paint(dst *ebiten.Image, vs []ebiten.Vertex, is []uint16, c color.NRGBA) {
	r, g, b, a := float32(c.R)/255, float32(c.G)/255, float32(c.B)/255, float32(c.A)/255
	for i := range vs {
		vs[i].SrcX, vs[i].SrcY = 1, 1
		vs[i].ColorR, vs[i].ColorG, vs[i].ColorB, vs[i].ColorA = r, g, b, a
	}
	op := &ebiten.DrawTrianglesOptions{AntiAlias: true, FillRule: ebiten.FillRuleNonZero}
	dst.DrawTriangles(vs, is, whiteSub, op)
}

func fillPath(dst *ebiten.Image, p *vector.Path, c color.NRGBA) {
	vs, is := p.AppendVerticesAndIndicesForFilling(nil, nil)
	paint(dst, vs, is, c)
}

func strokePath(dst *ebiten.Image, p *vector.Path, w float64, c color.NRGBA) {
	op := &vector.StrokeOptions{Width: float32(w), LineJoin: vector.LineJoinRound, LineCap: vector.LineCapRound}
	vs, is := p.AppendVerticesAndIndicesForStroke(nil, nil, op)
	paint(dst, vs, is, c)
}

func rect(dst *ebiten.Image, x, y, w, h float64, c color.NRGBA) {
	vector.DrawFilledRect(dst, float32(x), float32(y), float32(w), float32(h), c, false)
}

func circle(dst *ebiten.Image, x, y, r float64, c color.NRGBA) {
	vector.DrawFilledCircle(dst, float32(x), float32(y), float32(r), c, true)
}

func line(dst *ebiten.Image, x0, y0, x1, y1, w float64, c color.NRGBA) {
	vector.StrokeLine(dst, float32(x0), float32(y0), float32(x1), float32(y1), float32(w), c, true)
}

func roundRectPath(x, y, w, h, r float64) *vector.Path {
	var p vector.Path
	fx, fy, fw, fh, fr := float32(x), float32(y), float32(w), float32(h), float32(r)
	p.MoveTo(fx+fr, fy)
	p.LineTo(fx+fw-fr, fy)
	p.ArcTo(fx+fw, fy, fx+fw, fy+fr, fr)
	p.LineTo(fx+fw, fy+fh-fr)
	p.ArcTo(fx+fw, fy+fh, fx+fw-fr, fy+fh, fr)
	p.LineTo(fx+fr, fy+fh)
	p.ArcTo(fx, fy+fh, fx, fy+fh-fr, fr)
	p.LineTo(fx, fy+fr)
	p.ArcTo(fx, fy, fx+fr, fy, fr)
	p.Close()
	return &p
}

func roundRect(dst *ebiten.Image, x, y, w, h, r float64, c color.NRGBA) {
	fillPath(dst, roundRectPath(x, y, w, h, r), c)
}

// flameShape builds a teardrop flame whose base sits at (cx, baseY).
func flameShape(cx, baseY, w, h, sway float64) *vector.Path {
	var p vector.Path
	f := func(v float64) float32 { return float32(v) }
	tipX := cx + sway
	tipY := baseY - h
	p.MoveTo(f(tipX), f(tipY))
	p.CubicTo(f(cx+w*0.12+sway*0.6), f(baseY-h*0.62), f(cx+w*0.5), f(baseY-h*0.48), f(cx+w*0.5), f(baseY-h*0.26))
	p.CubicTo(f(cx+w*0.5), f(baseY-h*0.02), f(cx+w*0.26), f(baseY), f(cx), f(baseY))
	p.CubicTo(f(cx-w*0.26), f(baseY), f(cx-w*0.5), f(baseY-h*0.02), f(cx-w*0.5), f(baseY-h*0.26))
	p.CubicTo(f(cx-w*0.5), f(baseY-h*0.48), f(cx-w*0.12+sway*0.6), f(baseY-h*0.62), f(tipX), f(tipY))
	p.Close()
	return &p
}

var (
	flameOuter = hex("#E8410C")
	flameMid   = hex("#FF8C1A")
	flameInner = hex("#FFD23F")
)

// drawFlame draws a three-layer flickering flame.
var (
	firePal = [3]color.NRGBA{flameOuter, flameMid, flameInner}
	bluePal = [3]color.NRGBA{hex("#1F6FE0"), hex("#4FB2FF"), hex("#D8F3FF")}
)

// drawFlame draws a three-layer flickering flame.
func drawFlame(dst *ebiten.Image, cx, baseY, w, h, t, seed, alpha float64) {
	drawFlamePal(dst, cx, baseY, w, h, t, seed, alpha, firePal)
}

func drawFlamePal(dst *ebiten.Image, cx, baseY, w, h, t, seed, alpha float64, pal [3]color.NRGBA) {
	if h <= 0.5 || w <= 0.5 {
		return
	}
	s1 := math.Sin(t*11+seed)*0.10*w + math.Sin(t*17.3+seed*2)*0.05*w
	s2 := math.Sin(t*13+seed+1) * 0.08 * w
	s3 := math.Sin(t*19+seed+2) * 0.06 * w
	hf := 1 + 0.06*math.Sin(t*15+seed)
	fillPath(dst, flameShape(cx, baseY, w, h*hf, s1), withAlpha(pal[0], alpha))
	fillPath(dst, flameShape(cx, baseY, w*0.70, h*0.72*hf, s2), withAlpha(pal[1], alpha))
	fillPath(dst, flameShape(cx, baseY, w*0.40, h*0.42*hf, s3), withAlpha(pal[2], alpha))
}

// drawLog draws a rotated log centered at (cx, cy).
func drawLog(dst *ebiten.Image, cx, cy, length, thick, ang float64, bark, end, ring color.NRGBA) {
	dx, dy := math.Cos(ang)*length/2, math.Sin(ang)*length/2
	nx, ny := -math.Sin(ang)*thick/2, math.Cos(ang)*thick/2
	var p vector.Path
	p.MoveTo(float32(cx-dx+nx), float32(cy-dy+ny))
	p.LineTo(float32(cx+dx+nx), float32(cy+dy+ny))
	p.LineTo(float32(cx+dx-nx), float32(cy+dy-ny))
	p.LineTo(float32(cx-dx-nx), float32(cy-dy-ny))
	p.Close()
	fillPath(dst, &p, bark)
	circle(dst, cx-dx, cy-dy, thick/2, bark)
	circle(dst, cx+dx, cy+dy, thick/2, end)
	circle(dst, cx+dx, cy+dy, thick/4, ring)
}

func drawText(dst *ebiten.Image, s string, face *text.GoTextFace, x, y float64, align text.Align, c color.NRGBA) {
	op := &text.DrawOptions{}
	op.GeoM.Translate(x, y)
	op.ColorScale.ScaleWithColor(c)
	op.PrimaryAlign = align
	op.SecondaryAlign = text.AlignCenter
	text.Draw(dst, s, face, op)
}

var glowImg = func() *ebiten.Image {
	const n = 128
	img := image.NewRGBA(image.Rect(0, 0, n, n))
	for y := 0; y < n; y++ {
		for x := 0; x < n; x++ {
			dx, dy := (float64(x)+0.5)/n*2-1, (float64(y)+0.5)/n*2-1
			d := math.Sqrt(dx*dx + dy*dy)
			a := 0.0
			if d < 1 {
				a = math.Pow(1-d, 1.8)
			}
			v := uint8(a * 255)
			img.Set(x, y, color.RGBA{v, v, v, v})
		}
	}
	return ebiten.NewImageFromImage(img)
}()

// glow draws a smooth radial light; strength ~ the old per-ring alpha.
func glow(dst *ebiten.Image, x, y, r float64, c color.NRGBA, strength float64) {
	op := &ebiten.DrawImageOptions{}
	op.GeoM.Scale(2*r/128, 2*r/128)
	op.GeoM.Translate(x-r, y-r)
	op.ColorScale.ScaleWithColor(c)
	op.ColorScale.ScaleAlpha(float32(math.Min(1, strength*7)))
	op.Filter = ebiten.FilterLinear
	dst.DrawImage(glowImg, op)
}
