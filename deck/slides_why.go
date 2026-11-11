package main

import "github.com/austinlparker/decker"

// bothSlide is the honest comparison: what OBI gives you with no code, and
// what still takes an SDK.
func bothSlide() decker.Slide {
	const title = "Breadth and depth"
	sides := []struct {
		head  string
		col   func(*decker.Theme) decker.RGB
		lines []string
	}{
		{"OBI gives you", func(t *decker.Theme) decker.RGB { return t.Accent },
			[]string{"every service", "every network call", "no code changes"}},
		{"An SDK adds", func(t *decker.Theme) decker.RGB { return t.Accent2 },
			[]string{"your business data", "your own spans", "traces across queues"}},
	}
	return decker.Slide{Title: title, Section: "The trade", Steps: 2, Sources: []decker.Source{obiDocs},
		View: func(c decker.Ctx, sc *decker.Scene) {
			p, th := sc.Px, c.Theme
			body := heading(c, p, title)
			pad := c.Unit(0.035)
			cols := body.Cols(c.X(0.03), 1, 1)
			inner := cols[0].W - 2*pad - c.Unit(0.012)

			// One size for both panels: the largest at which the longest
			// line of either still fits its panel.
			head := decker.Text{Font: th.Display, Size: c.Size(0.105)}
			line := decker.Text{Font: th.Body, Size: c.Size(0.09), Color: th.Text}
			for _, s := range sides {
				head = head.Fit(s.head, inner, c.Y(0.13))
				for _, l := range s.lines {
					line = line.Fit(l, inner, c.Y(0.11))
				}
			}
			_, hh := head.Measure("Ag")
			_, lh := line.Measure("Ag")
			rowH := lh * 1.35
			panelH := 2*pad + hh*1.5 + 3*rowH
			c.Fits("comparison", body, body.W, panelH)
			top := body.Anchor(body.W, panelH, 0.5, 0.4)

			for i, s := range sides {
				r := decker.NewRect(cols[i].X, top.Y, cols[i].W, panelH)
				col := s.col(th)
				appear(c, p, i, 0, r, func(p *decker.Pixels) {
					p.RoundRect(r.X, r.Y, r.W, r.H, c.Unit(0.025), 0, th.Panel, 1)
					p.Rect(r.X, r.Y+c.Unit(0.025), c.Unit(0.012), r.H-2*c.Unit(0.025), col, 1)
					h := head
					h.Color = col
					x := r.X + pad + c.Unit(0.012)
					h.Draw(p, s.head, x, r.Y+pad)
					for j, l := range s.lines {
						line.Draw(p, l, x, r.Y+pad+hh*1.5+rowH*float64(j))
					}
				})
			}
		}}
}

// deploySlide shows the one rule behind every way of running OBI: it goes
// wherever it can share a kernel with what it watches.
func deploySlide() decker.Slide {
	const title = "Where OBI runs"
	hosts := []struct{ name, unit, how string }{
		{"Linux host", "app", "a process"},
		{"Docker", "app", "a container"},
		{"Kubernetes", "pod", "a DaemonSet"},
	}
	return decker.Slide{Title: title, Steps: len(hosts), Sources: []decker.Source{
		{Label: "Run OBI as a standalone process", URL: "https://opentelemetry.io/docs/zero-code/obi/setup/standalone/"},
		{Label: "Run OBI as a Docker container", URL: "https://opentelemetry.io/docs/zero-code/obi/setup/docker/"},
		{Label: "Deploy OBI in Kubernetes", URL: "https://opentelemetry.io/docs/zero-code/obi/setup/kubernetes/"},
	},
		View: func(c decker.Ctx, sc *decker.Scene) {
			p, th := sc.Px, c.Theme
			body := heading(c, p, title)
			c = zoom(c, 1.1)
			_, nh := nodeSize(c, "Ag")
			lh := lineHeight(c)
			pad := c.Unit(0.02)
			wUnit, wOBI := nodeWidth(c, "app", "pod"), nodeWidth(c, "OBI")
			kernelH := lh * 1.3
			hostW := 2*wUnit + 3*pad
			hostH := lh + pad + 2*(nh+pad) + kernelH + pad
			gap := c.X(0.025)
			n := float64(len(hosts))
			c.Fits("deployment diagram", body, n*hostW+(n-1)*gap, hostH+pad+lh)
			area := body.Anchor(n*hostW+(n-1)*gap, hostH+pad+lh, 0.5, 0.4)

			small := decker.Text{Font: th.Body, Size: c.SmallText(th.Body), Color: th.Muted, Align: decker.Center}
			for i, h := range hosts {
				if !c.Reached(i) {
					continue
				}
				r := decker.NewRect(area.X+float64(i)*(hostW+gap), area.Y, hostW, hostH)
				zone(c, p, r, h.name)
				kernel := decker.NewRect(r.X+pad, r.Bottom()-pad-kernelH, r.W-2*pad, kernelH)
				band(c, p, kernel, decker.Ease(c.Since(i)-0.3, 0.5))
				small.DrawMid(p, "kernel", kernel.X+kernel.W/2, kernel.Y+kernel.H/2)
				x, y := r.X+pad, r.Y+lh+pad
				for j := range 2 {
					unit := decker.NewRect(x, y, wUnit, nh)
					node(c, p, unit, h.unit, runtimes[(2*i+j)%len(runtimes)].col)
					x += wUnit + pad
				}
				// OBI sits on the kernel it shares with the workloads above.
				obi := decker.NewRect(r.X+(r.W-wOBI)/2, y+nh+pad, wOBI, nh)
				appear(c, p, i, 0.3, obi, func(p *decker.Pixels) { node(c, p, obi, "OBI", th.Accent) })
				caption(c, p, h.how, decker.NewRect(r.X, r.Bottom()+pad, r.W, lh), th.Accent, decker.Center, i, 0.5)
			}
		}}
}

func trySlide() decker.Slide {
	const title = "Try it"
	rows := []string{
		"github.com/austinlparker/obi-demo",
		"make vm && make up",
		"opentelemetry.io/docs/zero-code/obi",
	}
	return decker.Slide{Title: title, Section: "Try it", Sources: []decker.Source{obiDocs, obiBlog},
		View: func(c decker.Ctx, sc *decker.Scene) {
			p, th := sc.Px, c.Theme
			body := heading(c, p, title)
			c = zoom(c, 1.4)
			value := decker.Text{Font: th.Mono, Size: c.Size(0.1), Color: th.Text}
			for _, r := range rows {
				value = value.Fit(r, body.W, c.Y(0.1))
			}
			_, vh := value.Measure("Ag")
			rowH := vh * 1.5
			const bye = "May the traces be with you."
			last := decker.Text{Font: th.Display, Size: value.Size, Color: colHolo}.Fit(bye, body.W, c.Y(0.12))
			c.Fits("links", body, body.W, 4.3*rowH)
			y := body.Y + (body.H-4.3*rowH)*0.4
			for i, r := range rows {
				v := value
				if i == 1 {
					v.Color = th.Accent
				}
				v.FX = decker.FadeUp(c.T-0.15*float64(i)-0.1, 0.4, v.Size)
				v.Draw(p, r, body.X, y)
				y += rowH
			}
			last.FX = decker.FadeUp(c.T-0.9, 0.5, last.Size)
			last.Draw(p, bye, body.X, y+0.3*rowH)
		}}
}
