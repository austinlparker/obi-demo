package main

import (
	"math"

	"github.com/austinlparker/decker"
)

// shopSlide draws the request path through the shop, one hop per build. Every
// edge is labeled with the protocol OBI parses on it; the last build swaps
// each service's name for the language it is written in.
func shopSlide() decker.Slide {
	const title = "The demo: a swag shop"
	return decker.Slide{Title: title, Steps: 5,
		View: func(c decker.Ctx, sc *decker.Scene) {
			p, th := sc.Px, c.Theme
			body := heading(c, p, title)
			c = zoom(c, 1.05)

			// Three columns, each as wide as its widest name. The gateway
			// spans the first two, over the cart and the catalog; the wide
			// gap before the third holds a protocol label.
			w0 := nodeWidth(c, "cart", "Redis", "Node.js")
			w1 := nodeWidth(c, "catalog", "Postgres", "Python")
			w2 := nodeWidth(c, "checkout", "notifier")
			_, nh := nodeSize(c, "gateway")
			gapS, minGapL, minGapV := c.X(0.02), c.X(0.14), c.Y(0.12)
			c.Fits("shop diagram", body, w0+gapS+w1+minGapL+w2, 3*nh+2*minGapV)
			gapL := min(max(body.W-w0-gapS-w1-w2, minGapL), c.X(0.26))
			gapV := min(max((body.H-3*nh)/2, minGapV), c.Y(0.16))
			area := body.Anchor(w0+gapS+w1+gapL+w2, 3*nh+2*gapV, 0.5, 0.5)
			xs := []float64{area.X, area.X + w0 + gapS, area.X + w0 + gapS + w1 + gapL}
			ws := []float64{w0, w1, w2}
			at := func(col, row int) decker.Rect {
				return decker.NewRect(xs[col], area.Y+float64(row)*(nh+gapV), ws[col], nh)
			}
			gateway := decker.NewRect(xs[0], area.Y, w0+gapS+w1, nh)
			cart, catalog := at(0, 1), at(1, 1)
			redis, postgres := at(0, 2), at(1, 2)
			checkout, notifier := at(2, 0), at(2, 1)
			// over is the part of the gateway above a box in the row below,
			// so the arrow down to it is vertical.
			over := func(r decker.Rect) decker.Rect { return decker.NewRect(r.X, gateway.Y, r.W, gateway.H) }

			link(c, p, decker.Connector{From: over(cart), To: cart, Label: "HTTP"}, 1, 0)
			link(c, p, decker.Connector{From: over(catalog), To: catalog, Label: "HTTP"}, 1, 0.1)
			link(c, p, decker.Connector{From: cart, To: redis, Label: "RESP"}, 1, 0.4)
			link(c, p, decker.Connector{From: catalog, To: postgres, Label: "SQL"}, 1, 0.5)
			link(c, p, decker.Connector{From: gateway, To: checkout, Label: "gRPC"}, 2, 0)
			link(c, p, decker.Connector{From: checkout, To: notifier, Label: "NATS"}, 3, 0)
			link(c, p, decker.Connector{From: notifier, To: catalog, Label: "HTTP"}, 3, 0.5)

			boxes := []struct {
				r          decker.Rect
				name, lang string
				edge       decker.RGB
				step       int
				delay      float64
			}{
				{gateway, "gateway", "Go", colGateway, 0, 0.2},
				{cart, "cart", "Node.js", colCart, 1, 0.2},
				{catalog, "catalog", "Python", colCatalog, 1, 0.3},
				{redis, "Redis", "", th.Muted, 1, 0.6},
				{postgres, "Postgres", "", th.Muted, 1, 0.7},
				{checkout, "checkout", "Java", colCheckout, 2, 0.3},
				{notifier, "notifier", "Rust", colNotifier, 3, 0.3},
			}
			for _, b := range boxes {
				name := b.name
				if b.lang != "" && c.Reached(4) {
					name = b.lang
				}
				appear(c, p, b.step, b.delay, b.r, func(p *decker.Pixels) { node(c, p, b.r, name, b.edge) })
			}
		}}
}

// obiConfig is config/obi/obi.yaml cut down to the three decisions the slide
// walks through. The real file has a rule for each of the five services, the
// exporters and the route patterns.
const obiConfig = `capture:
  policy:
    default_action: exclude
  rules:
    - action: include
      match:
        process:
          open_ports: "8088"
  engine:
    propagation:
      context_propagation: all`

func configSlide() decker.Slide {
	const title = "The whole setup"
	return decker.Slide{Title: title, Section: "The demo", Steps: 4,
		View: func(c decker.Ctx, sc *decker.Scene) {
			body := heading(c, sc.Px, title)
			c = zoom(c, 1.05)
			// The file is taller than the slide, so it scrolls to each range.
			decker.Code{
				Source: obiConfig, Lang: "yaml", Title: "obi.yaml (abridged)",
				Focus:     []decker.LineRange{{From: 2, To: 3}, {From: 4, To: 8}, {From: 9, To: 11}},
				FirstStep: 1,
				Overflow:  decker.CodeScroll,
			}.Draw(c, sc.Px, body)
		}}
}

// demoSlide stays up while the demo runs in another window: the droid plays
// the shop's request path on a loop.
func demoSlide() decker.Slide {
	return decker.Slide{Title: "Demo",
		View: func(c decker.Ctx, sc *decker.Scene) {
			p := sc.Px
			stars(c, p)
			holo := c.Rect(0.27, 0.07, 0.69, 0.8)
			dh := c.Y(0.32)
			dx, base := c.X(0.13), c.Y(0.92)
			lx, ly := droidLens(dx, base, dh, false)
			on := decker.Ease(c.T-0.2, 0.6) * flicker(c.T)
			holoBeam(p, lx, ly, holo, on)

			inner := holo.Inset(c.X(0.04), c.Y(0.06))
			letters, graph := inner.CutTop(inner.H * 0.58)
			holoBlock(c, p, letters, [][]string{{"DEMO"}}, [][]decker.RGB{{colHolo}})

			// One request after another crossing the five services.
			n := len(runtimes)
			y := graph.Y + graph.H*0.55
			rad := min(c.Unit(0.045), graph.W/float64(6*n))
			xs := make([]float64, n)
			for i := range xs {
				xs[i] = graph.X + graph.W*(0.08+0.84*float64(i)/float64(n-1))
			}
			p.Line(xs[0], y, xs[n-1], y, max(c.Unit(0.006), 1), colHolo, 0.5)
			const period = 2.4
			for k := range 3 {
				u := math.Mod(c.T/period+float64(k)/3, 1)
				x := xs[0] + (xs[n-1]-xs[0])*u
				p.Glow(x, y, rad*1.6, colHoloLight, 0.6)
				p.Disc(x, y, rad*0.45, colHoloLight, 1)
			}
			for i, x := range xs {
				p.Disc(x, y, rad, colSpace, 1)
				p.Disc(x, y, rad, runtimes[i].col, 0.9)
				p.Disc(x, y, rad*0.5, colSpace, 1)
			}

			holoScan(c, p, holo)
			droid(c, p, dx, base, dh, false, on)
		}}
}
