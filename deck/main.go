// Command deck is the companion talk to a live demo of this repository, built on decker
// (https://github.com/austinlparker/decker). Run it from this directory:
//
//	go run .            # present
//	go run . present    # presenter view with notes
//	go run . review review
package main

import (
	_ "embed"

	"github.com/austinlparker/decker"
)

//go:embed notes.md
var notes string

func main() { decker.Main(talk()) }

func talk() decker.Deck {
	return decker.Deck{Name: "obi-demo", Theme: theme, Notes: notes, Slides: []decker.Slide{
		crawlSlide(),
		titleSlide(),
		problemSlide(),
		shopSlide(),
		otelSlide(),
		traceSlide(),
		sdkSlide(),
		ebpfSlide(),
		obiSlide(),
		propagationSlide(),
		configSlide(),
		demoSlide(),
		bothSlide(),
		deploySlide(),
		trySlide(),
	}}
}
