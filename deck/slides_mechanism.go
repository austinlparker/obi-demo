package main

import "github.com/austinlparker/decker"

// ebpfSlide introduces eBPF: the kernel is the one place every program's I/O
// passes through, and eBPF runs small programs there.
func ebpfSlide() decker.Slide {
	const title = "eBPF: code in the kernel"
	return decker.Slide{Title: title, Section: "The idea", Steps: 2,
		View: func(c decker.Ctx, sc *decker.Scene) {
			p, th := sc.Px, c.Theme
			body := heading(c, p, title)
			c = zoom(c, 1.3)
			_, nh := nodeSize(c, "Ag")
			dropH, pad := c.Y(0.1), c.Unit(0.02)
			bandH := nh + 2*pad
			need := nh + dropH + bandH
			c.Fits("eBPF diagram", body, body.W, need)
			area := body.Anchor(body.W, need, 0.5, 0.4)
			row, rest := area.CutTop(nh)
			_, rest = rest.CutTop(dropH)
			kernel, _ := rest.CutTop(bandH)

			names := make([]string, len(runtimes))
			for i, r := range runtimes {
				names[i] = r.name
			}
			hooked := decker.Ease(c.Since(1), 0.5)
			band(c, p, kernel, hooked)
			wProg, _ := nodeSize(c, "eBPF")
			prog := decker.NewRect(kernel.Right()-pad-wProg, kernel.Y+pad, wProg, nh)
			decker.Text{Font: th.Body, Size: c.SmallText(th.Body), Color: th.Text}.
				DrawMid(p, "Linux kernel", kernel.X+pad*1.5, kernel.Y+kernel.H/2)

			// Every runtime's I/O drops into the kernel; the eBPF program hooks
			// each crossing.
			boxes := nodeRow(c, "runtime row", row, c.X(0.015), c.X(0.05), names...)
			drop := decker.Ease(c.T-0.6, 0.6)
			for i, r := range boxes {
				x := r.X + r.W/2
				if drop > 0 {
					p.DashedLine(x, r.Bottom(), x, r.Bottom()+dropH*drop, max(c.Unit(0.006), 1), c.Unit(0.012), c.Unit(0.01),
						decker.Mix(th.Muted, th.Accent, hooked), 1)
				}
				if hooked > 0 {
					p.Disc(x, kernel.Y, c.Unit(0.014)*hooked, th.Accent, 1)
				}
				appear(c, p, 0, 0.08*float64(i), r, func(p *decker.Pixels) { node(c, p, r, names[i], runtimes[i].col) })
			}
			appear(c, p, 1, 0.2, prog, func(p *decker.Pixels) { node(c, p, prog, "eBPF", th.Accent) })
		}}
}

// obiSlide is the mechanism: a request between two services passes through
// the kernel, probes record it there, and OBI turns that into telemetry.
func obiSlide() decker.Slide {
	const title = "How OBI sees a request"
	return decker.Slide{Title: title, Steps: 4,
		View: func(c decker.Ctx, sc *decker.Scene) {
			p, th := sc.Px, c.Theme
			body := heading(c, p, title)
			c = zoom(c, 1.1)
			_, nh := nodeSize(c, "Ag")
			dropH, bandH, gapV := c.Y(0.07), c.Y(0.17), c.Y(0.09)
			need := nh + dropH + bandH + gapV + nh
			wGate, wCat := nodeWidth(c, "gateway"), nodeWidth(c, "catalog")
			wOBI, wCol := nodeWidth(c, "OBI"), nodeWidth(c, "Collector")
			c.Fits("OBI mechanism", body, wGate+wCat+wOBI+c.X(0.2), need)
			c.Fits("OBI output", body, wOBI+2*(c.X(0.15)+wCol), nh)
			area := body.Anchor(body.W, need, 0.5, 0.4)

			gateway := decker.NewRect(area.X, area.Y, wGate, nh)
			catalog := decker.NewRect(area.Right()-wCat, area.Y, wCat, nh)
			kernel := decker.NewRect(area.X, gateway.Bottom()+dropH, area.W, bandH)
			obi := decker.NewRect(area.X+(area.W-wOBI)/2, kernel.Bottom()+gapV, wOBI, nh)
			collector := decker.NewRect(area.Right()-wCol, obi.Y, wCol, nh)

			seen := decker.Ease(c.Since(1), 0.5)
			band(c, p, kernel, seen)
			decker.Text{Font: th.Body, Size: c.SmallText(th.Body), Color: th.Muted, Align: decker.Center}.
				DrawMid(p, "Linux kernel", kernel.X+kernel.W/2, kernel.Y+kernel.H*0.68)

			// The request: out of the gateway, through the kernel, into the
			// catalog.
			gx, kx := gateway.X+gateway.W/2, catalog.X+catalog.W/2
			wire := kernel.Y + kernel.H*0.24
			trail(c, p, []float64{gx, gateway.Bottom(), gx, wire, kx, wire, kx, catalog.Bottom()},
				th.Text, decker.Ease(c.T-0.5, 1.0))

			// A probe where the request is sent and one where it arrives.
			probes := []float64{gx + (kx-gx)*0.25, gx + (kx-gx)*0.75}
			for i, x := range probes {
				at := decker.NewRect(x-c.Unit(0.012), kernel.Bottom()-c.Unit(0.01), c.Unit(0.024), c.Unit(0.01))
				if seen > 0 {
					p.Glow(x, wire, c.Unit(0.05)*seen, th.Accent, 0.6)
					p.Disc(x, wire, c.Unit(0.018)*seen, th.Accent, 1)
				}
				k := decker.Connector{From: at, To: obi, FromSide: decker.SideBottom, ToSide: decker.SideTop,
					Color: th.Accent, Dashed: true}
				link(c, p, k, 2, 0.1*float64(i))
			}
			link(c, p, decker.Connector{From: obi, To: collector, Label: "OTLP"}, 3, 0.1)

			node(c, p, gateway, "gateway", colGateway)
			node(c, p, catalog, "catalog", colCatalog)
			appear(c, p, 2, 0.2, obi, func(p *decker.Pixels) { node(c, p, obi, "OBI", th.Accent) })
			appear(c, p, 3, 0.4, collector, func(p *decker.Pixels) { node(c, p, collector, "Collector", th.Accent2) })

		}}
}

// propagationSlide shows how hops join into one trace with no code: OBI
// writes the trace header on the way out and reads it on the way in.
func propagationSlide() decker.Slide {
	const title = "Joining the hops"
	const request, header = "GET /api/products", "traceparent: 00-4bf9...-01"
	return decker.Slide{Title: title, Steps: 2,
		View: func(c decker.Ctx, sc *decker.Scene) {
			p, th := sc.Px, c.Theme
			body := heading(c, p, title)
			c = zoom(c, 1.35)
			_, nh := nodeSize(c, "Ag")
			lh := lineHeight(c)
			mono := decker.Text{Font: th.Mono, Size: c.SmallText(th.Mono), Color: th.Muted}
			hw, mh := mono.Measure(header)
			pad, gapV := c.Unit(0.03), c.Y(0.05)
			panelW, panelH := hw+2*pad, 2*mh+2*pad
			need := nh + gapV + panelH + c.Y(0.02) + lh
			wGate, wCat := nodeWidth(c, "gateway"), nodeWidth(c, "catalog")
			c.Fits("propagation diagram", body, max(panelW, wGate+wCat+c.X(0.2)), need)
			area := body.Anchor(max(panelW, min(body.W, c.X(0.7))), need, 0.5, 0.4)

			gateway := decker.NewRect(area.X, area.Y, wGate, nh)
			catalog := decker.NewRect(area.Right()-wCat, area.Y, wCat, nh)
			link(c, p, decker.Connector{From: gateway, To: catalog, Label: "HTTP"}, 0, 0.3)
			node(c, p, gateway, "gateway", colGateway)
			node(c, p, catalog, "catalog", colCatalog)

			// The request on the wire: the app's line, then the one OBI adds.
			panel := decker.NewRect(area.X+(area.W-panelW)/2, gateway.Bottom()+gapV, panelW, panelH)
			p.RoundRect(panel.X, panel.Y, panel.W, panel.H, c.Unit(0.02), 0, th.Panel, 1)
			added := decker.Ease(c.Since(1), 0.4)
			p.RoundRect(panel.X, panel.Y, panel.W, panel.H, c.Unit(0.02), max(c.Unit(0.006), 1),
				decker.Mix(th.Faint, th.Accent, added), 1)
			mono.Draw(p, request, panel.X+pad, panel.Y+pad)
			if c.Reached(1) {
				line := mono
				line.Color = th.Accent
				line.FX = decker.TypeOn(c.Since(1)-0.2, 40)
				line.Draw(p, header, panel.X+pad, panel.Y+pad+mh)
			}

			// The one label the picture can't carry: who wrote that line.
			text := decker.NewRect(panel.X, panel.Bottom()+c.Y(0.02), panel.W-pad, lh)
			caption(c, p, "added by OBI", text, th.Accent, decker.Right, 1, 0.9)
		}}
}
