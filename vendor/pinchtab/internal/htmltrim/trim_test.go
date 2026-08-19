package htmltrim

import (
	"strings"
	"testing"
	"unicode/utf8"

	"golang.org/x/net/html"
)

func TestTrimHTMLStripsScriptsAndStyles(t *testing.T) {
	html := `<html><head><style>body{color:red}</style><script>var secret=1;</script></head>` +
		`<body><!-- a comment --><svg><path d="M0 0"/></svg><img src="data:image/png;base64,AAAA">` +
		`<button id="go">Go</button></body></html>`

	got := TrimHTML(html)

	for _, unwanted := range []string{"<style", "<script", "color:red", "var secret", "<!--", "<svg", "base64"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("trimmed HTML still contains %q:\n%s", unwanted, got)
		}
	}
	if !strings.Contains(got, `<button id="go">Go</button>`) {
		t.Errorf("trimmed HTML dropped the interactive element:\n%s", got)
	}
}

func TestTrimHTMLCapsOnRuneBoundary(t *testing.T) {
	body := strings.Repeat("é", maxTrimmedBytes)
	got := TrimHTML("<p>" + body + "</p>")

	if len(got) > maxTrimmedBytes {
		t.Fatalf("trimmed HTML is %d bytes, over the %d-byte cap", len(got), maxTrimmedBytes)
	}
	if len(got) < maxTrimmedBytes-utf8.UTFMax {
		t.Fatalf("trimmed HTML is %d bytes, far under the %d-byte cap", len(got), maxTrimmedBytes)
	}
	if !utf8.ValidString(got) {
		t.Fatalf("trimmed HTML is not valid UTF-8: %q", got[len(got)-8:])
	}
	if strings.ContainsRune(got, utf8.RuneError) {
		t.Fatalf("trimmed HTML contains U+FFFD absent from the source")
	}
	if strings.HasSuffix(got, ".") {
		t.Fatalf("trimmed HTML got a truncation marker appended: %q", got[len(got)-8:])
	}
	if !strings.HasPrefix("<p>"+body+"</p>", got) {
		t.Fatalf("trimmed HTML is not a byte-exact prefix of the source")
	}
}

func TestTrimHTMLLeavesShortInputUncapped(t *testing.T) {
	html := "<p>héllo</p>"
	if got := TrimHTML(html); got != html {
		t.Fatalf("TrimHTML(%q) = %q, want it unchanged", html, got)
	}
}

// Stripping has to happen before the cap, and neither of the assertions above can
// tell. Cap first and no script survives (stripping still runs) and the result is
// still under the cap — both stay green while the prompt budget is spent on
// content that is then thrown away, which is the token waste this package exists
// to remove. On a page whose interactive markup sits after a large script or style
// block, that markup is not merely crowded out, it is absent: the model is asked
// to act on a page with no form in it.
func TestTrimHTMLSpendsTheBudgetOnMarkupNotOnStrippedContent(t *testing.T) {
	html := "<html><head><style>" + strings.Repeat("body{color:red}", 200) +
		"</style><script>" + strings.Repeat("var x=1;doSomething();", 300) +
		"</script></head><body>" +
		`<form><input id="user" name="user"><input id="pass" type="password">` +
		`<button id="go">Sign in</button></form>` +
		"</body></html>"

	if len(html) <= maxTrimmedBytes {
		t.Fatalf("fixture is %d bytes, must exceed the %d-byte cap or the ordering is not exercised", len(html), maxTrimmedBytes)
	}

	got := TrimHTML(html)

	// The interactive markup is the whole reason the prompt carries HTML at all.
	for _, want := range []string{`id="go"`, `type="password"`, `id="user"`} {
		if !strings.Contains(got, want) {
			t.Errorf("trimmed HTML lost %s — the cap was applied before stripping, so the budget went on script and style that were then discarded:\n%s", want, got)
		}
	}
}

// dataURIGuards names each guard in reDataURI against the corruption it alone prevents.
// Three rounds of tuning this one pattern each traded one corruption for another, because
// a guard added for one round's cases reads as redundant to the next round's editor. Every
// guard must be named by at least one row of dataURICases, so deleting the last row for a
// guard reds here rather than going quiet.
var dataURIGuards = map[string]string{
	"leading delimiter":        `without it "Metadata:text/html,x" is cut to "Meta"`,
	"delimiter includes >":     `without it a payload opening an element's text content survives whole`,
	"delimiter includes ;":     `without it the ';' ending &quot; is unlicensed and the escaped CSS payload survives whole`,
	"mediatype letter-initial": `without it a comma-separated pair of dates reads as a mediatype and "Data:30/07,31/07" is deleted`,
	"mediatype comma required": `without it every other compact "Data:<value>" is deleted, and "Data:" is the word for date in five languages`,
	"entity-safe payload end":  `without it url(&quot;data:…&quot;) loses its closing entity and the declaration stops parsing`,
}

const dataURIPayload = "QUJDREVGR0hJSktMTU5PUFFSU1RVVldYWVo="

// dataURICases is the whole discovered class list for reDataURI in one table: every carrier
// a payload arrives in, and every prose shape that must survive. One table rather than a
// test per round, so a change to the pattern has to satisfy all of them at once.
var dataURICases = []struct {
	name string
	html string
	want string
	pins []string
}{
	{
		name: "attribute value, whole",
		html: `<img src="data:image/png;base64,` + dataURIPayload + `">`,
		want: `<img src="">`,
	},
	{
		name: "css url, unquoted",
		html: `<div style="background:url(data:image/png;base64,` + dataURIPayload + `)">x</div>`,
		want: `<div style="background:url()">x</div>`,
	},
	{
		name: "css url, single-quoted",
		html: `<div style="background-image:url('data:image/png;base64,` + dataURIPayload + `')">x</div>`,
		want: `<div style="background-image:url('')">x</div>`,
	},
	{
		name: "css url, escaped double-quoted, the only shape serialisation emits for it",
		html: `<div style="background:url(&quot;data:image/png;base64,` + dataURIPayload + `&quot;)">x</div>`,
		want: `<div style="background:url(&quot;&quot;)">x</div>`,
		pins: []string{"delimiter includes ;", "entity-safe payload end"},
	},
	{
		name: "css url among other declarations",
		html: `<div style="color:red;background:url(data:image/gif;base64,` + dataURIPayload + `);border:1px">x</div>`,
		want: `<div style="color:red;background:url();border:1px">x</div>`,
	},
	{
		name: "element text content",
		html: `<td>data:image/png;base64,` + dataURIPayload + `</td>`,
		want: `<td></td>`,
		pins: []string{"delimiter includes >"},
	},
	{
		name: "element text content, no base64",
		html: `<code>data:text/html,hello</code>`,
		want: `<code></code>`,
		pins: []string{"delimiter includes >"},
	},
	{
		name: "srcset candidate list",
		html: `<img srcset="data:image/png;base64,AAAABBBBCCCC 1x, data:image/png;base64,DDDDEEEEFFFF 2x" alt="pic">`,
		want: `<img srcset=" 1x, 2x" alt="pic">`,
	},
	{
		name: "empty mediatype, rfc-valid",
		html: `<td>data:,hello</td>`,
		want: `<td></td>`,
		pins: []string{"delimiter includes >"},
	},
	{
		name: "parameters only, rfc-valid",
		html: `<td>data:;base64,QUJD</td>`,
		want: `<td></td>`,
		pins: []string{"delimiter includes >"},
	},
	{
		name: "parameter with a value",
		html: `<td>data:text/plain;charset=utf-8,hello</td>`,
		want: `<td></td>`,
		pins: []string{"delimiter includes >"},
	},
	{
		name: "uppercase",
		html: `<td>DATA:IMAGE/PNG;BASE64,QUJD</td>`,
		want: `<td></td>`,
		pins: []string{"delimiter includes >"},
	},
	{
		name: "percent-encoded payload",
		html: `<td>data:text/plain,%20hi%21</td>`,
		want: `<td></td>`,
		pins: []string{"delimiter includes >"},
	},
	{
		name: "bare label",
		html: `<td>Data:</td>`,
		want: `<td>Data:</td>`,
		pins: []string{"mediatype comma required"},
	},
	{
		name: "spaced label",
		html: `<td>Data: 42</td>`,
		want: `<td>Data: 42</td>`,
		pins: []string{"mediatype comma required"},
	},
	{
		name: "compact label with a date",
		html: `<td>Data:30/07/2026</td>`,
		want: `<td>Data:30/07/2026</td>`,
		pins: []string{"mediatype comma required"},
	},
	{
		name: "compact label with a date, mid-sentence",
		html: `<p>Fattura Data:30/07/2026 totale</p>`,
		want: `<p>Fattura Data:30/07/2026 totale</p>`,
		pins: []string{"mediatype comma required"},
	},
	{
		name: "compact label with two dates",
		html: `<td>Data:30/07,31/07</td>`,
		want: `<td>Data:30/07,31/07</td>`,
		pins: []string{"mediatype letter-initial"},
	},
	{
		name: "compact label with a comma list",
		html: `<td>Data:1,2,3</td>`,
		want: `<td>Data:1,2,3</td>`,
		pins: []string{"mediatype comma required"},
	},
	{
		name: "compact label with a semicolon pair",
		html: `<td>Data:a;b</td>`,
		want: `<td>Data:a;b</td>`,
		pins: []string{"mediatype comma required"},
	},
	{
		name: "compact label with a single-letter path",
		html: `<td>Data:a/b</td>`,
		want: `<td>Data:a/b</td>`,
		pins: []string{"mediatype comma required"},
	},
	{
		name: "data: inside a word",
		html: `<p>Metadata: 2024</p>`,
		want: `<p>Metadata: 2024</p>`,
	},
	{
		name: "data: inside a word with a mime tail",
		html: `<td>metadata:image/png here</td>`,
		want: `<td>metadata:image/png here</td>`,
	},
	{
		name: "data: inside a word with a whole uri after it",
		html: `<td>Metadata:text/html,x</td>`,
		want: `<td>Metadata:text/html,x</td>`,
		pins: []string{"leading delimiter"},
	},
	{
		name: "data- attribute",
		html: `<div data-src="keep">x</div>`,
		want: `<div data-src="keep">x</div>`,
	},
}

func TestTrimHTMLHandlesEveryDataURICarrierAndProseShape(t *testing.T) {
	for _, tc := range dataURICases {
		t.Run(tc.name, func(t *testing.T) {
			got := TrimHTML(tc.html)
			if got != tc.want {
				t.Errorf("TrimHTML(%q) =\n  %s\nwant\n  %s", tc.html, got, tc.want)
			}
			if strings.Contains(got, dataURIPayload) {
				t.Errorf("the base64 payload survived and spends the whole prompt budget:\n%s", got)
			}
		})
	}
}

// The escaped row above is hand-written with the entity Chrome's serialiser emits. This
// derives the same shape by running a double-quoted CSS url() through a serialiser instead,
// so the row is pinned against what serialisation actually produces rather than against an
// approximation of it — and against either entity spelling a serialiser may choose.
func TestTrimHTMLStripsThePayloadASerialiserEscapedItself(t *testing.T) {
	div := &html.Node{
		Type: html.ElementNode,
		Data: "div",
		Attr: []html.Attribute{
			{Key: "id", Val: "d"},
			{Key: "style", Val: `background:url("data:image/png;base64,` + dataURIPayload + `")`},
		},
	}
	div.AppendChild(&html.Node{Type: html.TextNode, Data: "styled"})

	var rendered strings.Builder
	if err := html.Render(&rendered, div); err != nil {
		t.Fatalf("render: %v", err)
	}

	serialised := rendered.String()
	if strings.Contains(serialised, `url("data:`) {
		t.Fatalf("the serialiser left a bare quote, so this fixture is not the escaped shape:\n%s", serialised)
	}
	if !strings.Contains(serialised, dataURIPayload) {
		t.Fatalf("the serialiser dropped the payload, so nothing is left to strip:\n%s", serialised)
	}

	open := strings.Index(serialised, "url(") + len("url(")
	entity := serialised[open:strings.Index(serialised, "data:")]

	got := TrimHTML(serialised)
	if strings.Contains(got, dataURIPayload) {
		t.Errorf("the payload survived the shape serialisation actually emits:\n%s", got)
	}
	if want, have := strings.Count(serialised, entity), strings.Count(got, entity); have != want {
		t.Errorf("the strip took %d of the %d %s that quote the url — the declaration no longer parses:\n%s",
			want-have, want, entity, got)
	}
	if !strings.Contains(got, `)">styled`) {
		t.Errorf("the declaration stopped being readable:\n%s", got)
	}
}

func TestEveryDataURIGuardIsPinnedByACase(t *testing.T) {
	pinned := map[string]int{}
	for _, tc := range dataURICases {
		for _, guard := range tc.pins {
			if _, ok := dataURIGuards[guard]; !ok {
				t.Errorf("case %q pins %q, which is not a declared guard", tc.name, guard)
				continue
			}
			pinned[guard]++
		}
	}
	for guard, corruption := range dataURIGuards {
		if pinned[guard] == 0 {
			t.Errorf("no case pins the %q guard, so removing it corrupts silently: %s", guard, corruption)
		}
	}
}

// SVG is foreign content and nests legally, so a nested <svg> reaches this helper through
// DOM serialisation. The non-greedy regex stopped at the FIRST </svg>, leaving the outer
// element's path data and a stray closing tag in the prompt.
func TestTrimHTMLStripsNestedSVGWhole(t *testing.T) {
	got := TrimHTML(`<svg>a<svg>b</svg>PATHDATA</svg>tail`)

	for _, unwanted := range []string{"<svg", "</svg", "PATHDATA"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("nested SVG left %q behind: %q", unwanted, got)
		}
	}
	if got != "tail" {
		t.Errorf("TrimHTML = %q, want %q", got, "tail")
	}
}

// Depth counting must not become a greedy match: greedy runs to the LAST </svg> in the
// document and would swallow every interactive element between two sibling icons, which
// is worse than the nesting bug it fixes.
func TestTrimHTMLKeepsMarkupBetweenSiblingSVGs(t *testing.T) {
	got := TrimHTML(`<svg><path d="M0 0"/></svg><button id="go">Go</button><svg><circle r="1"/></svg>`)

	if !strings.Contains(got, `<button id="go">Go</button>`) {
		t.Errorf("markup between two sibling SVGs was swallowed: %q", got)
	}
	for _, unwanted := range []string{"<svg", "</svg", "<path", "<circle"} {
		if strings.Contains(got, unwanted) {
			t.Errorf("sibling SVG left %q behind: %q", unwanted, got)
		}
	}
}

// An element whose name merely starts with "svg" is ordinary markup and must survive; the
// boundary check is what separates <svg> from <svgicon>.
func TestTrimHTMLKeepsElementsMerelyNamedLikeSVG(t *testing.T) {
	html := `<svgicon id="keep">label</svgicon>`
	if got := TrimHTML(html); got != html {
		t.Errorf("TrimHTML(%q) = %q, want it unchanged", html, got)
	}
}

// PINNED BOUNDARY, not a defect: a raw-text element ends at the FIRST </script per the
// HTML tokenizer, so the tail after it is page-visible text that the browser renders too
// — golang.org/x/net/html picks byte-for-byte the same boundary. Do not "fix" this.
func TestTrimHTMLLeavesTheTailAfterAScriptStringLiteral(t *testing.T) {
	got := TrimHTML(`<p>before</p><script>var s="</script>";var secret=2;</script><p>after</p>`)

	if strings.Contains(got, "var s=") {
		t.Errorf("script content reached the prompt: %q", got)
	}
	if !strings.Contains(got, `";var secret=2;`) {
		t.Errorf("the tail is page text the browser also renders and must survive: %q", got)
	}
	for _, want := range []string{"<p>before</p>", "<p>after</p>"} {
		if !strings.Contains(got, want) {
			t.Errorf("surrounding markup lost %q: %q", want, got)
		}
	}
}
