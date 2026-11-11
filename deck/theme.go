package main

import (
	"math"
	"strings"

	"github.com/austinlparker/decker"
)

// One color per shop service, used wherever a service is drawn.
var (
	colGateway  = decker.Hex("#5BC8F5")
	colCatalog  = decker.Hex("#FFD966")
	colCart     = decker.Hex("#8CD96B")
	colCheckout = decker.Hex("#FF8F5C")
	colNotifier = decker.Hex("#C9A0FF")
)

// The five runtimes, in the colors their services have in every diagram.
var runtimes = []struct {
	name string
	col  decker.RGB
}{{"Go", colGateway}, {"Python", colCatalog}, {"Node.js", colCart}, {"Java", colCheckout}, {"Rust", colNotifier}}

// heroFont is the block lettering for the title and the demo beat.
var heroFont = decker.StockFigletFont("Decker Sign")

// The hologram and the droid that projects it. colCrawl is the yellow of the
// opening crawl.
var (
	colHolo      = decker.Hex("#6FD8FF")
	colHoloLight = decker.Hex("#D6F4FF")
	colCrawl     = decker.Hex("#FFD23F")
	colDroid     = decker.Hex("#DCE4EE")
	colDroidDome = decker.Hex("#9FB1C8")
	colDroidTrim = decker.Hex("#3F6FE0")
	colSpace     = decker.Hex("#05070D")
)

var theme = &decker.Theme{
	Background: decker.Hex("#0D1117"),
	Text:       decker.Hex("#EEF2F8"),
	Muted:      decker.Hex("#97A3B6"),
	Faint:      decker.Hex("#2A3344"),
	Accent:     decker.Hex("#F5A800"),
	Accent2:    decker.Hex("#7C96FF"),
	Warn:       decker.Hex("#FF6B81"),
	Good:       decker.Hex("#7EDC9A"),
	Panel:      decker.Hex("#161C28"),
	Series:     []decker.RGB{colGateway, colCatalog, colCart, colCheckout, colNotifier},
	Display:    decker.StockFont("SpaceGrotesk-Bold"),
	Body:       decker.StockFont("SpaceGrotesk-Medium"),
	Mono:       decker.StockFont("JetBrainsMono-ExtraBold"),
	Overlay: func(c decker.Ctx, p *decker.Pixels) {
		th := c.Theme
		decker.ProgressBar(c, p, decker.NewRect(0, c.PH()-c.Unit(0.008), c.PW(), c.Unit(0.008)), th.Accent, th.Faint)
	},
}

// heading draws a slide's title and returns the rect left for its content:
// the page inside its margins, below the title and above the corner droid.
func heading(c decker.Ctx, p *decker.Pixels, title string) decker.Rect {
	th := c.Theme
	page := c.Frame().Inset(c.X(0.04), c.Y(0.05))
	_, page = page.CutBottom(c.Y(0.125))
	head, body := page.CutTop(c.Y(0.11))
	cornerDroid(c, p, body)
	t := decker.Text{Font: th.Display, Size: c.Size(0.095), Color: th.Text}.Fit(title, head.W, head.H)
	t.FX = decker.RiseIn(c.T, 0.02, t.Size)
	_, h := t.Draw(p, title, head.X, head.Y)
	rule := c.X(0.05) * decker.EaseOutExpo(decker.Progress(c.T, 0.1, 0.6))
	p.Rect(head.X, head.Y+h+c.Unit(0.006), rule, c.Unit(0.008), th.Accent, 1)
	_, body = body.CutTop(c.Y(0.03))
	return body
}

// zoom returns c as if the canvas were k times larger, so everything a slide
// sizes from it (text, boxes, gaps) grows by k while the rect it lays out in
// stays the real one. Each slide zooms as far as its diagram still fits at
// the smallest terminal the review checks.
func zoom(c decker.Ctx, k float64) decker.Ctx {
	c.W, c.H = int(float64(c.W)*k), int(float64(c.H)*k)
	return c
}

// nodeSize is the size of a diagram box that holds name on one line.
func nodeSize(c decker.Ctx, name string) (w, h float64) {
	tw, th := decker.Text{Font: c.Theme.Display, Size: c.SmallText(c.Theme.Display)}.Measure(name)
	return tw + 2*c.Unit(0.03), th * 1.6
}

// nodeWidth is the width of a box wide enough for any of names.
func nodeWidth(c decker.Ctx, names ...string) float64 {
	w := 0.0
	for _, n := range names {
		nw, _ := nodeSize(c, n)
		w = max(w, nw)
	}
	return w
}

// node draws a box in a diagram: a panel with a colored edge and a name.
func node(c decker.Ctx, p *decker.Pixels, r decker.Rect, name string, edge decker.RGB) {
	th := c.Theme
	rad := min(c.Unit(0.02), r.H/3)
	p.RoundRect(r.X, r.Y, r.W, r.H, rad, 0, th.Panel, 1)
	p.RoundRect(r.X, r.Y, r.W, r.H, rad, max(c.Unit(0.006), 1), edge, 1)
	decker.Text{Font: th.Display, Size: c.SmallText(th.Display), Color: th.Text, Align: decker.Center}.
		DrawMid(p, name, r.X+r.W/2, r.Y+r.H/2)
}

// appear pops an element in on its build step, delay seconds after the step
// begins.
func appear(c decker.Ctx, p *decker.Pixels, step int, delay float64, r decker.Rect, draw func(p *decker.Pixels)) {
	decker.AppearAt(c, step, -1, func(t float64) decker.Composite {
		return decker.Pop(t-delay, 0.35)
	}, nil).Draw(p, r, draw)
}

// link draws an arrow between two boxes on its build step.
func link(c decker.Ctx, p *decker.Pixels, k decker.Connector, step int, delay float64) {
	k.Head = decker.HeadArrow
	k.Prog = decker.Ease(c.Since(step)-delay, 0.5)
	k.Draw(c, p)
}

// caption writes a small line of text that fades up on its build step.
func caption(c decker.Ctx, p *decker.Pixels, s string, r decker.Rect, col decker.RGB, align decker.Align, step int, delay float64) {
	th := c.Theme
	small := c.SmallText(th.Body)
	x := r.X
	switch align {
	case decker.Center:
		x = r.X + r.W/2
	case decker.Right:
		x = r.Right()
	}
	decker.Text{Font: th.Body, Size: small, Color: col, Align: align, MaxW: r.W,
		FX: decker.FadeUp(c.Since(step)-delay, 0.4, small)}.Draw(p, s, x, r.Y)
}

// cornerDroid puts the droid in the bottom-right corner of a content slide,
// projecting the slide's body. It is drawn before the content, so the beam
// lies under the diagrams like light, and it doesn't move: a beam that
// flickered behind every diagram would pull the eye off them.
func cornerDroid(c decker.Ctx, p *decker.Pixels, body decker.Rect) {
	h := c.Y(0.14)
	lx, ly := droidLens(c.PW()-c.X(0.035)-h*0.36, c.PH()-c.Y(0.03), h, true)
	// A fan of light toward the middle of the body, laid down in layers so
	// it is brightest at the lens and gone before it crosses the content.
	bx, by := body.Center()
	aim := math.Atan2(by-ly, bx-lx)
	const spread, layers = 0.3, 5
	for i := 1; i <= layers; i++ {
		l := c.Y(0.55) * float64(i) / layers
		p.Polygon([]float64{lx, ly,
			lx + l*math.Cos(aim-spread), ly + l*math.Sin(aim-spread),
			lx + l*math.Cos(aim+spread), ly + l*math.Sin(aim+spread)}, colHolo, 0.028)
	}
	droid(c, p, c.PW()-c.X(0.035)-h*0.36, c.PH()-c.Y(0.03), h, true, 1)
}

// droidLens is where the projector lens of a droid of height h sits, standing
// with its feet centered on (cx, base). The lens is on the right of the dome,
// or the left if flip.
func droidLens(cx, base, h float64, flip bool) (x, y float64) {
	dx := 0.11 * h
	if flip {
		dx = -dx
	}
	return cx + dx, base - 0.74*h
}

// droid draws a small astromech of height h standing with its feet centered on
// (cx, base). lit, 0..1, is how brightly its projector lens glows.
func droid(c decker.Ctx, p *decker.Pixels, cx, base, h float64, flip bool, lit float64) {
	bw, top := 0.48*h, base-0.62*h
	for _, side := range []float64{-1, 1} {
		lx := cx + side*(bw/2+0.065*h)
		p.RoundRect(lx-0.05*h, top+0.06*h, 0.1*h, 0.5*h, 0.03*h, 0, colDroid, 1)
		p.Rect(lx-0.05*h, top+0.2*h, 0.1*h, 0.07*h, colDroidTrim, 1)
		p.RoundRect(lx-0.085*h, base-0.09*h, 0.17*h, 0.09*h, 0.025*h, 0, colDroid, 1)
	}
	p.Rect(cx-0.07*h, base-0.07*h, 0.14*h, 0.07*h, colDroidDome, 1)
	p.Disc(cx, top, bw/2, colDroidDome, 1)
	p.RoundRect(cx-bw/2, top, bw, 0.54*h, 0.03*h, 0, colDroid, 1)
	p.Rect(cx-bw/2, top, bw, 0.035*h, colDroidTrim, 1)
	p.Rect(cx-0.15*h, top+0.1*h, 0.3*h, 0.07*h, colDroidTrim, 1)
	p.Rect(cx-0.05*h, top+0.22*h, 0.1*h, 0.24*h, colDroidTrim, 1)
	p.Rect(cx-0.19*h, top+0.24*h, 0.08*h, 0.2*h, colDroidDome, 1)
	p.Rect(cx+0.11*h, top+0.24*h, 0.08*h, 0.2*h, colDroidDome, 1)

	lx, ly := droidLens(cx, base, h, flip)
	ex := 2*cx - lx - (lx-cx)*0.3
	p.Disc(ex, base-0.75*h, 0.05*h, colSpace, 1)
	p.Disc(ex, base-0.75*h, 0.02*h, c.Theme.Warn, 1)
	p.Glow(lx, ly, 0.12*h*lit, colHolo, 0.7*lit)
	p.Disc(lx, ly, 0.032*h, decker.Mix(colDroidTrim, colHoloLight, lit), 1)
}

// stars scatters a fixed starfield over the canvas.
func stars(c decker.Ctx, p *decker.Pixels) {
	p.Fill(colSpace)
	n := int(c.PW() * c.PH() / 900)
	for i := range n {
		x := int(decker.Hash01(i, 1, 7) * c.PW())
		y := int(decker.Hash01(i, 2, 7) * c.PH())
		p.Blend(x, y, colHoloLight, 0.25+0.6*decker.Hash01(i, 3, 7))
	}
}

// flicker is a projector's unsteady brightness at time t, between 0.8 and 1.
func flicker(t float64) float64 {
	return 0.8 + 0.2*decker.Hash01(int(t*14), 0, 3)
}

// holoBeam draws the light from a lens at (lx, ly) to the near edge of the
// hologram r, and the faint plate the hologram hangs on.
func holoBeam(p *decker.Pixels, lx, ly float64, r decker.Rect, bright float64) {
	near := []float64{lx, ly, r.X, r.Y, r.X, r.Bottom()}
	if lx > r.X && lx < r.Right() {
		near = []float64{lx, ly, r.X, r.Bottom(), r.Right(), r.Bottom()}
	}
	p.Polygon(near, colHolo, 0.1*bright)
	p.Polygon([]float64{lx, ly, r.X, r.Y, r.Right(), r.Y, r.Right(), r.Bottom(), r.X, r.Bottom()}, colHolo, 0.03*bright)
	p.RoundRect(r.X, r.Y, r.W, r.H, 0, 0, colHolo, 0.05*bright)
}

// holoScan lays a projector's scanlines over the hologram r, with one bright
// line drifting down it. Draw it after the hologram's content.
func holoScan(c decker.Ctx, p *decker.Pixels, r decker.Rect) {
	step := max(c.Unit(0.018), 3)
	for y := r.Y; y < r.Bottom(); y += step {
		p.Rect(r.X, y, r.W, 1, colSpace, 0.35)
	}
	y := r.Y + r.H*math.Mod(c.T*0.12, 1)
	p.Rect(r.X, y, r.W, max(c.Unit(0.006), 1), colHoloLight, 0.18)
}

// holoBlock draws lines of block lettering as large as r allows, centered in
// r, and returns the rect it drew in. cols colors each word of each line; a
// line with fewer colors than words repeats its last.
func holoBlock(c decker.Ctx, p *decker.Pixels, r decker.Rect, lines [][]string, cols [][]decker.RGB) decker.Rect {
	b := decker.Block{Font: heroFont, Scale: 1, Glow: 0.6}
	joined := make([]string, len(lines))
	for i, l := range lines {
		joined[i] = strings.Join(l, " ")
	}
	w1, h1 := b.Size(strings.Join(joined, "\n"))
	b.Scale = max(math.Floor(2*min(r.W/w1, r.H/h1))/2, 0.5)
	w, h := b.Size(strings.Join(joined, "\n"))
	c.Fits("block lettering", r, w, h)
	out := r.Anchor(w, h, 0.5, 0.5)
	lineH := h / float64(len(lines))
	b.FX = decker.BlockDecrypt(c.T, 1.0, c.Theme.Faint)
	for i, words := range lines {
		lw, _ := b.Size(joined[i])
		x := out.X + (out.W-lw)/2
		space := 0.0
		if len(words) > 1 {
			sum := 0.0
			for _, wd := range words {
				ww, _ := b.Size(wd)
				sum += ww
			}
			space = (lw - sum) / float64(len(words)-1)
		}
		for j, wd := range words {
			b.Color = cols[i][min(j, len(cols[i])-1)]
			ww, _ := b.Draw(p, wd, x, out.Y+float64(i)*lineH)
			x += ww + space
		}
	}
	return out
}

// lineHeight is the height of one line of small body text.
func lineHeight(c decker.Ctx) float64 {
	return float64(c.SmallText(c.Theme.Body)) * decker.DefaultLeading
}

// statement draws one line of display type as large as r allows, centered in
// r, from its build step on.
func statement(c decker.Ctx, p *decker.Pixels, s string, r decker.Rect, col decker.RGB, step int, delay float64) {
	if !c.Reached(step) {
		return
	}
	t := decker.Text{Font: c.Theme.Display, Size: c.Size(0.12), Color: col, Align: decker.Center}.Fit(s, r.W, r.H)
	t.FX = decker.FadeUp(c.Since(step)-delay, 0.4, t.Size)
	t.DrawMid(p, s, r.X+r.W/2, r.Y+r.H/2)
}

// nodeRow lays out one box per name along the top of r, each as wide as its
// name, centered, with a gap between minGap and maxGap.
func nodeRow(c decker.Ctx, what string, r decker.Rect, minGap, maxGap float64, names ...string) []decker.Rect {
	_, nh := nodeSize(c, "Ag")
	ws := make([]float64, len(names))
	sum := 0.0
	for i, n := range names {
		ws[i], _ = nodeSize(c, n)
		sum += ws[i]
	}
	n := float64(len(names) - 1)
	c.Fits(what, r, sum+n*minGap, nh)
	gap := minGap
	if n > 0 {
		gap = min(max((r.W-sum)/n, minGap), maxGap)
	}
	x := r.X + (r.W-sum-n*gap)/2
	out := make([]decker.Rect, len(names))
	for i := range names {
		out[i] = decker.NewRect(x, r.Y, ws[i], nh)
		x += ws[i] + gap
	}
	return out
}

// band draws the kernel: a full-width bar whose edge lights up by lit, 0..1.
func band(c decker.Ctx, p *decker.Pixels, r decker.Rect, lit float64) {
	th := c.Theme
	p.RoundRect(r.X, r.Y, r.W, r.H, c.Unit(0.015), 0, th.Faint, 1)
	p.RoundRect(r.X, r.Y, r.W, r.H, c.Unit(0.015), max(c.Unit(0.006), 1), decker.Mix(th.Faint, th.Accent, lit), 1)
}

// zone outlines a group of boxes and names it on a line inside its top edge.
func zone(c decker.Ctx, p *decker.Pixels, r decker.Rect, label string) {
	th := c.Theme
	p.RoundRect(r.X, r.Y, r.W, r.H, c.Unit(0.02), max(c.Unit(0.006), 1), th.Muted, 1)
	decker.Label(c, p, label, r.X+c.Unit(0.02), r.Y+c.Unit(0.012), th.Muted, decker.Left)
}

// trail strokes the first prog (0..1) of the path through pts, given as x, y
// pairs, and puts an arrowhead on its end once it is complete.
func trail(c decker.Ctx, p *decker.Pixels, pts []float64, col decker.RGB, prog float64) {
	if prog <= 0 {
		return
	}
	total := 0.0
	for i := 0; i+3 < len(pts); i += 2 {
		total += math.Hypot(pts[i+2]-pts[i], pts[i+3]-pts[i+1])
	}
	w := max(c.Unit(0.008), 1)
	left := total * min(prog, 1)
	for i := 0; i+3 < len(pts); i += 2 {
		x0, y0, x1, y1 := pts[i], pts[i+1], pts[i+2], pts[i+3]
		d := math.Hypot(x1-x0, y1-y0)
		if last := i+4 >= len(pts); last && prog >= 1 {
			decker.Arrow(p, x0, y0, x1, y1, w, col, 1)
			return
		}
		f := min(left/d, 1)
		p.Line(x0, y0, x0+(x1-x0)*f, y0+(y1-y0)*f, w, col, 1)
		if left -= d; left <= 0 {
			return
		}
	}
}
