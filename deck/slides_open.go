package main

import (
	"math"
	"sync"

	"github.com/austinlparker/decker"
)

var obiBlog = decker.Source{
	Label: "OpenTelemetry eBPF Instrumentation (OBI) v1.0.0 release",
	URL:   "https://opentelemetry.io/blog/2026/obi-v1-release/",
}

var obiDocs = decker.Source{
	Label: "OpenTelemetry eBPF Instrumentation documentation",
	URL:   "https://opentelemetry.io/docs/zero-code/obi/",
}

// crawl is the opening text, one row per line; an empty row is a paragraph
// break. The first two rows are the episode heading.
var crawl = []string{
	"EPISODE 1.0",
	"THE KERNEL AWAKENS",
	"",
	"It is a period of slow",
	"requests. One click crosses",
	"five services, written in",
	"five languages by five teams.",
	"",
	"Each team sees only its own",
	"part. Some services cannot",
	"be changed at all.",
	"",
	"But deep in the kernel,",
	"a new hope stirs....",
}

// crawlTex is the crawl's text drawn flat, white on black, at twice the size
// it has at the bottom of the screen, so the projection has detail to sample.
// It depends only on the canvas size and the theme's font, so it is drawn once
// per size and kept.
type crawlTex struct {
	px   *decker.Pixels
	over float64 // texture pixels per screen pixel at the bottom edge
}

var crawlTexes sync.Map // [2]int canvas size -> crawlTex

func crawlTexture(c decker.Ctx) crawlTex {
	key := [2]int{c.W, c.H}
	if t, ok := crawlTexes.Load(key); ok {
		return t.(crawlTex)
	}
	const over = 2.0
	w := c.PW() * over
	t := decker.Text{Font: c.Theme.Display, Size: c.Size(0.2), Color: decker.Hex("#FFFFFF"), Align: decker.Center}
	for _, row := range crawl {
		t = t.Fit(row, w*0.96, c.Y(0.2))
	}
	rowH := float64(t.Size) * 1.45
	px := decker.NewPixels(int(w), int(rowH*float64(len(crawl))+rowH), decker.RGB{})
	for i, row := range crawl {
		t.DrawMid(px, row, w/2, rowH*(float64(i)+0.75))
	}
	tex := crawlTex{px: px, over: over}
	crawlTexes.Store(key, tex)
	return tex
}

// sample is the texture's coverage, 0..1, at (x, y), bilinear.
func (t crawlTex) sample(x, y float64) float64 {
	x0, y0 := int(math.Floor(x)), int(math.Floor(y))
	if x0 < 0 || y0 < 0 || x0+1 >= t.px.W || y0+1 >= t.px.H {
		return 0
	}
	fx, fy := float32(x-float64(x0)), float32(y-float64(y0))
	i := y0*t.px.W + x0
	a, b := t.px.Pix[i].R, t.px.Pix[i+1].R
	cc, d := t.px.Pix[i+t.px.W].R, t.px.Pix[i+t.px.W+1].R
	return float64((a*(1-fx)+b*fx)*(1-fy)+(cc*(1-fx)+d*fx)*fy) / 255
}

// crawlSlide is the cold open: the card, then the text receding into the
// stars. The text lies on a plane tilted away from the audience: each screen
// row below the horizon is a strip of that plane, wider and closer the lower
// it is. The crawl stops with every line still on screen, so the slide can be
// left up while the room reads it.
func crawlSlide() decker.Slide {
	const card = "A long time ago, in a cluster far, far away...."
	return decker.Slide{Title: "A long time ago", Steps: 2, HideChrome: true,
		View: func(c decker.Ctx, sc *decker.Scene) {
			p, th := sc.Px, c.Theme
			if !c.Reached(1) {
				p.Fill(colSpace)
				t := decker.Text{Font: th.Body, Size: c.Size(0.085), Color: colHolo, Align: decker.Center,
					MaxW: c.X(0.8)}
				t.FX = decker.FadeUp(c.T-0.3, 0.8, t.Size)
				_, h := t.Measure(card)
				t.Draw(p, card, c.X(0.5), (c.PH()-h)/2)
				return
			}
			stars(c, p)

			tex := crawlTexture(c)
			texH := float64(tex.px.H)
			horizon, floor := c.Y(0.04), c.PH()
			// The plane is wider than the screen where it meets the bottom
			// edge, so the nearest rows fill the frame, and tilt is how much
			// shorter than wide a letter looks there.
			const wide, tilt = 1.3, 0.6
			zoom := tex.over / wide // texture pixels per screen pixel at the bottom edge
			// depth is how far along the plane, in texture pixels, the view
			// reaches by the time a strip is half as wide as the bottom one.
			depth := zoom * (floor - horizon) / tilt
			// The text enters under the bottom edge and comes to rest with its
			// last row just clear of it, while the first is still readable.
			scroll := min(c.Since(1)*c.PH()*0.3*tex.over, texH+0.06*depth)
			cx := c.PW() / 2
			for y := int(horizon) + 1; y < p.H; y++ {
				s := (float64(y) + 0.5 - horizon) / (floor - horizon) // strip width, 0 at the horizon to 1
				ty := scroll - (1/s-1)*depth
				// One screen row covers span texture rows; average over them
				// so far rows blur instead of sparkling.
				span := depth / (s * s * (floor - horizon))
				if ty+span < 0 || ty >= texH {
					continue
				}
				ny := min(max(int(math.Ceil(span/2)), 1), 8)
				nx := min(max(int(math.Ceil(zoom/s/2)), 1), 6)
				fade := decker.Clamp01((s - 0.1) / 0.2)
				half := min(int(cx*s*wide)+1, int(cx))
				for x := int(cx) - half; x <= int(cx)+half; x++ {
					tx := float64(tex.px.W)/2 + (float64(x)+0.5-cx)*zoom/s
					sum := 0.0
					for j := range ny {
						for i := range nx {
							sum += tex.sample(
								tx+(float64(i)+0.5-float64(nx)/2)*zoom/s/float64(nx),
								ty+(float64(j)+0.5)*span/float64(ny))
						}
					}
					p.Blend(x, y, colCrawl, fade*sum/float64(nx*ny))
				}
			}
		}}
}

// titleSlide is the joke the talk is named for: the droid plays its message.
func titleSlide() decker.Slide {
	return decker.Slide{Title: "What Is OBI?", Sources: []decker.Source{obiBlog},
		View: func(c decker.Ctx, sc *decker.Scene) {
			p, th := sc.Px, c.Theme
			stars(c, p)
			holo := c.Rect(0.27, 0.07, 0.69, 0.8)
			dh := c.Y(0.32)
			dx, base := c.X(0.13), c.Y(0.92)
			lx, ly := droidLens(dx, base, dh, false)
			on := decker.Ease(c.T-0.2, 0.6) * flicker(c.T)
			holoBeam(p, lx, ly, holo, on)

			inner := holo.Inset(c.X(0.03), c.Y(0.05))
			letters, rest := inner.CutTop(inner.H * 0.66)
			holoBlock(c, p, letters,
				[][]string{{"HELP ME,"}, {"OBI", "WAN"}, {"TELEMETRY"}},
				[][]decker.RGB{{colHolo}, {th.Accent, colHolo}, {colHolo}})

			const plea = "You're my only hope!"
			t := decker.Text{Font: th.Display, Size: c.Size(0.1), Color: colHoloLight, Align: decker.Center}.
				Fit(plea, rest.W, rest.H*0.55)
			t.FX = decker.FadeUp(c.T-1.0, 0.5, t.Size)
			_, h := t.Draw(p, plea, rest.X+rest.W/2, rest.Y+c.Y(0.03))
			caption(c, p, "OpenTelemetry eBPF Instrumentation, v1.0",
				decker.NewRect(holo.X, rest.Y+c.Y(0.03)+h+c.Y(0.02), holo.W, rest.H), th.Muted, decker.Center, 0, 1.4)

			holoScan(c, p, holo)
			droid(c, p, dx, base, dh, false, on)
		}}
}
