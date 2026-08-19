// Package scroll owns the scroll DIRECTION KEYWORD: which words name a direction,
// and how far one of them moves. Every surface that accepts "down" reads it from
// here, because the vocabulary and the magnitude drifted apart the moment two
// surfaces each kept their own copy — the CLI moved 800px per keyword while MCP
// moved 120 and offered no horizontal keyword at all, so the same word meant
// different things depending on how an agent was talking to the browser.
//
// This is deliberately NOT the same notion as the browser's default wheel notch
// (internal/bridge defaultScrollNotch, 120), which answers a different question:
// what a wheel event with no delta does. A notch is a device unit; a keyword is an
// intent. Collapsing the two is what made a sixth of a viewport look like a
// reasonable answer to "scroll down", so the two values stay separate on purpose.
package scroll

import "sort"

// StepPixels is how far one direction keyword moves. It is a viewport-scale step
// rather than a device notch because a caller naming a direction means "bring the
// next content into view" — a notch takes six or seven round trips to do that, with
// a snapshot between each.
const StepPixels = 800

// Direction is where a keyword points: the request field that carries it and the
// signed distance. Axis is the wire spelling, so a caller can build a request body
// without restating the mapping.
type Direction struct {
	Axis  string
	Delta int
}

var directions = map[string]Direction{
	"down":  {Axis: "scrollY", Delta: StepPixels},
	"up":    {Axis: "scrollY", Delta: -StepPixels},
	"right": {Axis: "scrollX", Delta: StepPixels},
	"left":  {Axis: "scrollX", Delta: -StepPixels},
}

// DirectionFor resolves a keyword, which callers must treat as case-insensitive
// input from an agent.
func DirectionFor(keyword string) (Direction, bool) {
	d, ok := directions[keyword]
	return d, ok
}

// DirectionKeywords returns the accepted keywords, sorted. Prose that enumerates
// them — CLI help, MCP tool descriptions, agent references — is asserted against
// this rather than against a hand-kept list, so an invented keyword and an omitted
// one both fail.
func DirectionKeywords() []string {
	keywords := make([]string, 0, len(directions))
	for keyword := range directions {
		keywords = append(keywords, keyword)
	}
	sort.Strings(keywords)
	return keywords
}
