package main

import "github.com/austinlparker/decker"

// problemSlide is the question the rest of the talk answers: one request,
// five services, and nobody who can see all of it.
func problemSlide() decker.Slide {
	const title = "One click, five services"
	return decker.Slide{Title: title, Section: "The problem", Steps: 2,
		View: func(c decker.Ctx, sc *decker.Scene) {
			p, th := sc.Px, c.Theme
			body := heading(c, p, title)
			c = zoom(c, 1.2)
			_, nh := nodeSize(c, "Go")
			qH, gap := c.Y(0.16), c.Y(0.08)
			area := body.Anchor(body.W, nh+gap+qH, 0.5, 0.4)
			row, rest := area.CutTop(nh)
			_, rest = rest.CutTop(gap)
			question, _ := rest.CutTop(qH)

			names := make([]string, len(runtimes))
			for i, r := range runtimes {
				names[i] = r.name
			}
			boxes := nodeRow(c, "service chain", row, c.X(0.03), c.X(0.07), names...)
			lost := decker.Ease(c.Since(1), 0.5)
			for i, r := range boxes {
				if i > 0 {
					link(c, p, decker.Connector{From: boxes[i-1], To: r}, 0, 0.3+0.15*float64(i))
				}
				edge := decker.Mix(runtimes[i].col, th.Muted, lost*0.7)
				appear(c, p, 0, 0.15*float64(i), r, func(p *decker.Pixels) { node(c, p, r, names[i], edge) })
			}
			statement(c, p, "It's slow. Which one?", question, th.Warn, 1, 0.1)
		}}
}

// otelSlide introduces OpenTelemetry as three signals and one wire format.
func otelSlide() decker.Slide {
	const title = "OpenTelemetry"
	signals := []string{"traces", "metrics", "logs"}
	return decker.Slide{Title: title, Steps: 3,
		View: func(c decker.Ctx, sc *decker.Scene) {
			p, th := sc.Px, c.Theme
			body := heading(c, p, title)
			c = zoom(c, 1.2)
			_, nh := nodeSize(c, "Ag")
			gapV := c.Y(0.04)
			stackH := 3*nh + 2*gapV

			wApps, wSig, wBack := nodeWidth(c, "apps"), nodeWidth(c, signals...), nodeWidth(c, "any backend")
			gapS, minGapL := c.X(0.05), c.X(0.15)
			c.Fits("OpenTelemetry diagram", body, wApps+gapS+wSig+minGapL+wBack, stackH)
			gapS = min(max((body.W-wApps-wSig-wBack-minGapL)/2, gapS), c.X(0.1))
			gapL := min(max(body.W-wApps-wSig-wBack-gapS, minGapL), c.X(0.24))
			area := body.Anchor(wApps+gapS+wSig+gapL+wBack, stackH, 0.5, 0.4)
			mid := area.Y + (stackH-nh)/2
			apps := decker.NewRect(area.X, mid, wApps, nh)
			back := decker.NewRect(area.Right()-wBack, mid, wBack, nh)

			for i, s := range signals {
				r := decker.NewRect(apps.Right()+gapS, area.Y+float64(i)*(nh+gapV), wSig, nh)
				d := 0.1 * float64(i)
				link(c, p, decker.Connector{From: apps, To: r, Route: decker.RouteCurved}, 1, d)
				appear(c, p, 1, 0.2+d, r, func(p *decker.Pixels) { node(c, p, r, s, th.Accent2) })
			}
			// One arrow for all three: they share a wire format.
			stack := decker.NewRect(apps.Right()+gapS, mid, wSig, nh)
			link(c, p, decker.Connector{From: stack, To: back, Label: "OTLP"}, 2, 0)
			appear(c, p, 0, 0.1, apps, func(p *decker.Pixels) { node(c, p, apps, "apps", colGateway) })
			appear(c, p, 2, 0.3, back, func(p *decker.Pixels) { node(c, p, back, "any backend", th.Good) })
		}}
}

// traceSlide draws a trace as a waterfall, then makes one span slow.
func traceSlide() decker.Slide {
	const title = "One request, one trace"
	spans := []struct {
		name     string
		col      decker.RGB
		from, to float64
		slow     bool
	}{
		{"gateway", colGateway, 0, 1, false},
		{"cart", colCart, 0.03, 0.15, false},
		{"catalog", colCatalog, 0.17, 0.74, true},
		{"Postgres", decker.Hex("#97A3B6"), 0.2, 0.3, false},
		{"checkout", colCheckout, 0.76, 0.97, false},
	}
	return decker.Slide{Title: title, Steps: 3,
		View: func(c decker.Ctx, sc *decker.Scene) {
			p, th := sc.Px, c.Theme
			body := heading(c, p, title)
			c = zoom(c, 1.25)
			lh := lineHeight(c)
			rowH, gap := lh*1.15, c.Y(0.02)
			n := float64(len(spans))
			name := decker.Text{Font: th.Body, Size: c.SmallText(th.Body), Color: th.Text}
			labelW := 0.0
			for _, s := range spans {
				w, _ := name.Measure(s.name)
				labelW = max(labelW, w)
			}
			need := n*rowH + (n-1)*gap
			c.Fits("trace waterfall", body, labelW+c.X(0.3), need)
			area := body.Anchor(body.W, need, 0.5, 0.4)
			bars := decker.NewRect(area.X+labelW+c.X(0.02), area.Y, area.W-labelW-c.X(0.02), n*rowH+(n-1)*gap)

			found := decker.Ease(c.Since(2), 0.5)
			for i, s := range spans {
				step, delay := 0, 0.0
				if i > 0 {
					step, delay = 1, 0.12*float64(i)
				}
				if !c.Reached(step) {
					continue
				}
				y := area.Y + float64(i)*(rowH+gap)
				col, text := s.col, th.Text
				if !s.slow {
					col = decker.Mix(col, th.Background, found*0.65)
					text = decker.Mix(text, th.Background, found*0.5)
				}
				nm := name
				nm.Color = text
				nm.FX = decker.FadeUp(c.Since(step)-delay, 0.3, nm.Size)
				nm.DrawMid(p, s.name, area.X, y+rowH/2)
				grow := decker.Ease(c.Since(step)-delay, 0.6)
				x := bars.X + bars.W*s.from
				p.RoundRect(x, y+rowH*0.15, bars.W*(s.to-s.from)*grow, rowH*0.7, c.Unit(0.012), 0, col, 1)
			}
		}}
}

// sdkSlide shows how instrumentation usually gets into a service: a library
// inside the process, once per service and per language.
func sdkSlide() decker.Slide {
	const title = "The usual way: an SDK"
	return decker.Slide{Title: title, Steps: 4,
		View: func(c decker.Ctx, sc *decker.Scene) {
			p, th := sc.Px, c.Theme
			body := heading(c, p, title)
			c = zoom(c, 1.15)
			_, nh := nodeSize(c, "Ag")
			lh := lineHeight(c)
			pad, gapV := c.Unit(0.02), c.Y(0.09)
			zoneH := lh + nh + 2*pad

			wCode, wSDK, wBack := nodeWidth(c, "your code"), nodeWidth(c, "OTel SDK"), nodeWidth(c, "backend")
			zoneW := wCode + wSDK + 4*pad
			minGapL := c.X(0.15)
			need := zoneH + gapV + nh
			c.Fits("SDK diagram", body, zoneW+minGapL+wBack, need)
			gapL := min(max(body.W-zoneW-wBack, minGapL), c.X(0.22))
			area := body.Anchor(zoneW+gapL+wBack, need, 0.5, 0.4)

			proc := decker.NewRect(area.X, area.Y, zoneW, zoneH)
			code := decker.NewRect(proc.X+pad, proc.Y+lh+pad, wCode, nh)
			sdk := decker.NewRect(code.Right()+2*pad, code.Y, wSDK, nh)
			back := decker.NewRect(area.Right()-wBack, code.Y, wBack, nh)

			zone(c, p, proc, "one process")
			link(c, p, decker.Connector{From: sdk, To: back, Label: "OTLP"}, 1, 0.3)
			node(c, p, code, "your code", colGateway)
			appear(c, p, 1, 0, sdk, func(p *decker.Pixels) { node(c, p, sdk, "OTel SDK", th.Accent2) })
			appear(c, p, 1, 0.5, back, func(p *decker.Pixels) { node(c, p, back, "backend", th.Good) })

			// The same work again for every language, and then the service
			// nobody can change.
			names := make([]string, 0, len(runtimes)+1)
			for _, r := range runtimes {
				names = append(names, r.name)
			}
			names = append(names, "?")
			row := decker.NewRect(body.X, proc.Bottom()+gapV, body.W, nh)
			for i, r := range nodeRow(c, "language row", row, c.X(0.015), c.X(0.04), names...) {
				if i == len(runtimes) {
					appear(c, p, 3, 0, r, func(p *decker.Pixels) { node(c, p, r, "?", th.Warn) })
					continue
				}
				appear(c, p, 2, 0.1*float64(i), r, func(p *decker.Pixels) { node(c, p, r, names[i], runtimes[i].col) })
			}
		}}
}
