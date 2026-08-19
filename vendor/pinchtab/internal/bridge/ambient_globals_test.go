package bridge

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"sort"
	"strings"
	"testing"

	"github.com/pinchtab/pinchtab/internal/srccensus"
)

// An isolated-world node handle lives in the TOP frame's isolated context, so
// inside a Runtime.callFunctionOn declaration the ambient window/document are the
// top frame's, not the node's. A frame-offset walk started from the ambient window
// terminates on its first iteration and yields frame-local coordinates; a hit test
// there runs against the wrong document. Both fail silently.
//
// This census is the enforcement half of that rule. Its sibling,
// TestIsolatedWorldBoundaryCensus, covers WHICH element is acted on; this one
// covers what the declaration is allowed to read once it has one.
//
// PRESENCE, NOT ABSENCE. It does not try to prove a bare window/document is
// shadowed — that is an absence heuristic over JS inside Go string literals and
// fails open on any wording it did not anticipate, which is exactly the shape of
// the near miss it exists to catch (`const view = window;` landing under a comment
// asserting the opposite). Instead: if a declaration mentions an ambient global at
// all, it must carry a node-derived shadow line verbatim. A hand-rolled variant
// fails closed.
//
// GRANULARITY: per file, not per call site. Correlating a declaration with whether
// its handle came from IsolatedNodeObjectID is not reliably textual, so a file that
// obtains isolated handles puts every callFunctionOn declaration in it in scope.
//
// FALSE POSITIVES point one way, and that way is safe: a main-world declaration
// sitting in such a file is asked to shadow too. Shadowing is correct in either
// world — it costs one line and changes no behaviour. The census never excuses a
// declaration that should shadow; it can only ask one that need not to.
//
// THE CANONICAL LINES STAY INLINE, four copies of one line, deliberately. Hoisting
// them into a shared const would blind this census, because what it matches is text
// inside the literal it scans. Four copies held identical by a test that fails on
// drift beats one const the guard cannot see. If declaration assembly ever becomes
// first-class, revisit both together.

// isolatedHandleTokens put a file in scope: it either produces an isolated-world
// handle or obtains one. Naming the producers rather than a directory is what
// keeps the census module-wide — pinchtab spreads CDP work over internal/bridge,
// internal/bridge/cdpops and internal/cdptk.
// The list is hand-maintained, which makes it the census's one unguarded input: a
// third producer under a new name puts nothing in scope, so the guard narrows in
// silence — the scope counters below stay identical and the canonical cross-check
// does not fire, because the new file has no canonical line to miss.
// TestIsolatedHandleProducersAreAllInScope closes that by deriving the producer set
// from the CDP call that creates an isolated handle instead of from these names.
var isolatedHandleTokens = []string{"IsolatedNodeObjectID", "FrameExecutionContextID"}

// isolatedResolveWindow is how far after a DOM.resolveNode the isolating argument
// has to appear to belong to it. The resolution census uses the same span for the
// same reason: the two are one call written across a few lines. The spellings come
// from that census's resolveNodeSpellings table, so both guards share one
// definition of "an isolated resolve" by construction rather than by coincidence.
const isolatedResolveWindow = 200

// A file that resolves a node with an executionContextId produces a top-frame
// isolated handle, which is precisely what makes the ambient globals the wrong
// frame's. Every such file must therefore be in scope. Deriving the producers from
// the CDP call rather than trusting isolatedHandleTokens is what stops the list
// going stale: a new producer under any name fails here, naming itself, instead of
// quietly removing its own callers from the census.
func TestIsolatedHandleProducersAreAllInScope(t *testing.T) {
	producers := 0
	for _, file := range srccensus.Tree(t, "../..", moduleGoFileFloor) {
		name, src := file.Name, file.Text

		// Both spellings, from the one table the resolution census already owns. That
		// census added the typed pairing precisely because a check knowing only the
		// string literal reads as module-wide while every typed call site goes
		// unchecked; borrowing its window without its spellings would reproduce that
		// here, and this check's failure mode is silence.
		isolating := false
		for _, spelling := range resolveNodeSpellings {
			for _, block := range strings.Split(src, spelling.call)[1:] {
				head := block
				if len(head) > isolatedResolveWindow {
					head = head[:isolatedResolveWindow]
				}
				if strings.Contains(head, spelling.isolated) {
					isolating = true
					break
				}
			}
			if isolating {
				break
			}
		}
		if !isolating {
			continue
		}
		producers++

		inScope := false
		for _, token := range isolatedHandleTokens {
			if strings.Contains(src, token) {
				inScope = true
				break
			}
		}
		if inScope {
			continue
		}
		t.Errorf("%s resolves a node with an executionContextId, so its handles are top-frame isolated, but it matches no entry in isolatedHandleTokens — the ambient-globals census cannot see it or its callers. Add the producer's exported name to isolatedHandleTokens.", name)
	}
	if producers == 0 {
		t.Fatal("no file resolves a node with an executionContextId — this check is verifying nothing")
	}
	t.Logf("isolated-handle producers derived from the CDP call: %d, all in census scope", producers)
}

// shadowForm is one accepted way to derive a global from the node. requires is
// extra text that must also be present for the form to be node-derived at all:
// the root spelling is only sound because the declaration binds root to this.
type shadowForm struct {
	line     string
	requires string
}

// ambientShadows is the rule as data: mention the global, carry one of its forms.
// Entries are compliance, not exemption — every one derives the global from the
// node the handle points at.
var ambientShadows = map[string][]shadowForm{
	"window": {
		{line: "const view = (this.ownerDocument && this.ownerDocument.defaultView) || window;"},
	},
	"document": {
		{line: "const doc = this.ownerDocument || document;"},
		{line: "const document = root.ownerDocument || root;", requires: "const root = this;"},
	},
}

// ambientGlobalsExceptions is empty, and the first entry ever added has to be
// argued on the harm it accepts, not on how reasonable the wording sounds. It is
// checked in both directions like the resolution census: a listed file that no
// longer violates the rule fails here too, so an exception cannot outlive its
// reason.
var ambientGlobalsExceptions = map[string]string{}

// canonicalViewLine is the preamble as it stands in the tree. Its two counts below
// answer two different questions that one counter used to conflate:
//   - in-scope but unreached: a declaration this census does not recognise. It
//     detects a blind spot only for this one marker, so it is not the guarantee that
//     the walk reaches every literal — TestTheCensusWalkReachesEveryRawLiteralInTheModule
//     is, and it asserts the property this comment used to claim by construction.
//   - present in a file that is out of scope: the file shadows correctly but no
//     longer matches isolatedHandleTokens, so its declarations stopped being
//     scanned. That is a scope problem with a different remedy, and reporting it as
//     declaration assembly sends the reader to audit a parser that is working.
const canonicalViewLine = "const view = (this.ownerDocument && this.ownerDocument.defaultView) || window;"

// jsSegment records where one spliced fragment starts in the assembled text and
// which source line it came from, so an offset anywhere in a +-concatenated
// declaration maps back to a line the reader can open.
type jsSegment struct {
	offset int
	line   int
}

type jsLiteral struct {
	text      string
	startLine int
	segments  []jsSegment
}

// lineAt maps an offset in the assembled text to its source line, counting
// newlines only within the fragment that offset falls in. A spliced declaration
// does not lay out like its source, so counting from the start of the merged text
// reports a line that does not exist.
func (l jsLiteral) lineAt(offset int) int {
	line := l.startLine
	for _, seg := range l.segments {
		if seg.offset > offset {
			break
		}
		line = seg.line + strings.Count(l.text[seg.offset:offset], "\n")
	}
	return line
}

// rawLiteralText reports the contents of a raw (backtick) string literal.
func rawLiteralText(expr ast.Expr) (string, bool) {
	lit, ok := expr.(*ast.BasicLit)
	if !ok || lit.Kind != token.STRING || !strings.HasPrefix(lit.Value, "`") {
		return "", false
	}
	return strings.Trim(lit.Value, "`"), true
}

// identRawLiteral resolves an identifier used in a concatenation chain to the raw
// literal it is bound to, through the parser's own scope resolution — so two
// functions each binding the same name to a different fragment cannot splice the
// wrong text, which a file-global identifier map allowed.
func identRawLiteral(expr ast.Expr) (string, bool) {
	ident, ok := expr.(*ast.Ident)
	if !ok || ident.Obj == nil {
		return "", false
	}
	switch decl := ident.Obj.Decl.(type) {
	case *ast.AssignStmt:
		for i, lhs := range decl.Lhs {
			if name, ok := lhs.(*ast.Ident); ok && name.Name == ident.Name && i < len(decl.Rhs) {
				return rawLiteralText(decl.Rhs[i])
			}
		}
	case *ast.ValueSpec:
		for i, name := range decl.Names {
			if name.Name == ident.Name && i < len(decl.Values) {
				return rawLiteralText(decl.Values[i])
			}
		}
	}
	return "", false
}

// appendChain flattens a +-joined expression left to right, splicing raw literals
// and resolved named fragments. The fragment that reads a global is routinely not
// the fragment that opens the function — the xpath arm of the selector resolver
// lives in a tail fragment — so a chain has to be assembled, not sampled.
//
// It returns the operands it could NOT absorb — a call, an index, a conversion — in
// source order. The caller consumes the chain whole and must walk those itself:
// they can hold raw literals of their own, and a chain consumed without them is a
// literal recorded nowhere.
func appendChain(fset *token.FileSet, lit *jsLiteral, expr ast.Expr) []ast.Expr {
	if bin, ok := expr.(*ast.BinaryExpr); ok && bin.Op == token.ADD {
		return append(appendChain(fset, lit, bin.X), appendChain(fset, lit, bin.Y)...)
	}
	text, ok := rawLiteralText(expr)
	if !ok {
		if text, ok = identRawLiteral(expr); !ok {
			return []ast.Expr{expr}
		}
	}
	line := fset.Position(expr.Pos()).Line
	if lit.text == "" && lit.startLine == 0 {
		lit.startLine = line
	}
	lit.segments = append(lit.segments, jsSegment{offset: len(lit.text), line: line})
	lit.text += text
	return nil
}

// chainHasRawLiteral reports whether a +-chain contains a raw literal at all, so a
// chain of ordinary strings is not mistaken for an assembled JS declaration.
func chainHasRawLiteral(expr ast.Expr) bool {
	if bin, ok := expr.(*ast.BinaryExpr); ok && bin.Op == token.ADD {
		return chainHasRawLiteral(bin.X) || chainHasRawLiteral(bin.Y)
	}
	_, ok := rawLiteralText(expr)
	return ok
}

// goRawLiterals returns every raw string literal in a Go file, splicing
// +-concatenated ones — named fragments included, resolved in their own scope —
// into the single string the browser is actually handed.
func goRawLiterals(name, src string) ([]jsLiteral, error) {
	literals, _, err := goRawLiteralsWithReach(name, src)
	return literals, err
}

// goRawLiteralsWithReach also reports the byte offset of every raw literal the walk
// took in, which is what TestTheCensusWalkReachesEveryRawLiteralInTheModule compares
// against a naive every-literal census. Offsets rather than counts, so a literal
// swapped for another still reads as missed, and byte offsets rather than token.Pos
// so two separate parses of the same source are comparable.
func goRawLiteralsWithReach(name, src string) ([]jsLiteral, map[int]bool, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.ParseComments)
	if err != nil {
		return nil, nil, err
	}

	var literals []jsLiteral
	reached := map[int]bool{}
	var walk func(n ast.Node) bool
	walk = func(n ast.Node) bool {
		switch expr := n.(type) {
		case *ast.BinaryExpr:
			if expr.Op != token.ADD || !chainHasRawLiteral(expr) {
				return true
			}
			var lit jsLiteral
			skipped := appendChain(fset, &lit, expr)
			markRawLiteralsReached(fset, reached, expr, skipped)
			if lit.text != "" {
				literals = append(literals, lit)
			}
			// The chain is consumed whole, so its own parts must not be recorded again
			// — but an operand the chain could not absorb is walked explicitly. Letting
			// this return stand for those too is what made a raw literal inside a call
			// operand invisible to the census.
			for _, operand := range skipped {
				ast.Inspect(operand, walk)
			}
			return false
		case *ast.BasicLit:
			text, ok := rawLiteralText(expr)
			if !ok {
				return false
			}
			line := fset.Position(expr.Pos()).Line
			reached[fset.Position(expr.Pos()).Offset] = true
			literals = append(literals, jsLiteral{
				text:      text,
				startLine: line,
				segments:  []jsSegment{{offset: 0, line: line}},
			})
			return false
		}
		return true
	}
	ast.Inspect(file, walk)
	return literals, reached, nil
}

// markRawLiteralsReached records the raw literals a consumed chain absorbed: every one
// under the chain except those inside an operand the chain skipped, which is walked
// separately and records itself.
func markRawLiteralsReached(fset *token.FileSet, reached map[int]bool, chain ast.Expr, skipped []ast.Expr) {
	skippedNodes := map[ast.Expr]bool{}
	for _, operand := range skipped {
		skippedNodes[operand] = true
	}
	var mark func(expr ast.Expr)
	mark = func(expr ast.Expr) {
		if skippedNodes[expr] {
			return
		}
		if bin, ok := expr.(*ast.BinaryExpr); ok && bin.Op == token.ADD {
			mark(bin.X)
			mark(bin.Y)
			return
		}
		if _, ok := rawLiteralText(expr); ok {
			reached[fset.Position(expr.Pos()).Offset] = true
		}
	}
	mark(chain)
}

// declarations returns the literals that are Runtime.callFunctionOn function
// declarations: a bare function expression. An IIFE (`(function(`) is a
// Runtime.evaluate payload — no node handle, so the rule does not apply and the
// census never sees it. Excluded by scope beats excluded by exception.
func callFunctionDeclarations(literals []jsLiteral) []jsLiteral {
	var decls []jsLiteral
	for _, lit := range literals {
		head := strings.TrimLeft(lit.text, " \t\n")
		if strings.HasPrefix(head, "function(") || strings.HasPrefix(head, "function (") {
			decls = append(decls, lit)
		}
	}
	return decls
}

// ambientMention reports the offset of the first mention of name that is not a field
// access or part of a longer identifier, so ownerDocument and el.document never
// count as ambient reads.
func ambientMention(text, name string) int {
	for from := 0; ; {
		idx := strings.Index(text[from:], name)
		if idx < 0 {
			return -1
		}
		at := from + idx
		from = at + len(name)
		// A dot BEFORE the name is a field access (el.ownerDocument); a dot AFTER it
		// is the ambient read itself (document.evaluate), which is the whole point.
		// Treating both the same way is how a detector reports mentions it never
		// actually looks for.
		if at > 0 && (text[at-1] == '.' || isIdentByte(text[at-1])) {
			continue
		}
		if from < len(text) && isIdentByte(text[from]) {
			continue
		}
		return at
	}
}

func isIdentByte(b byte) bool {
	return b == '_' || b == '$' ||
		(b >= 'a' && b <= 'z') || (b >= 'A' && b <= 'Z') || (b >= '0' && b <= '9')
}

// jsCode blanks whole-line JS comments, keeping the line structure. It matters in
// both directions: prose mentioning window is not an ambient read, and a
// commented-out preamble must not satisfy the rule — that commenting-out is the
// exact near miss this census exists to catch. Only full-line comments are
// blanked, never a trailing one, because eating code would make the census fail
// open, and a trailing comment can only ever cost a harmless extra shadow.
func jsCode(text string) string {
	lines := strings.Split(text, "\n")
	for i, line := range lines {
		trimmed := strings.TrimSpace(line)
		for _, prefix := range []string{"//", "/*", "*/", "*"} {
			if strings.HasPrefix(trimmed, prefix) {
				lines[i] = strings.Repeat(" ", len(line))
				break
			}
		}
	}
	return strings.Join(lines, "\n")
}

func lineAtOffset(text string, offset int) string {
	start := strings.LastIndexByte(text[:offset], '\n') + 1
	end := strings.IndexByte(text[start:], '\n')
	if end < 0 {
		return strings.TrimSpace(text[start:])
	}
	return strings.TrimSpace(text[start : start+end])
}

func hasShadow(text string, forms []shadowForm) bool {
	for _, form := range forms {
		if !strings.Contains(text, form.line) {
			continue
		}
		if form.requires != "" && !strings.Contains(text, form.requires) {
			continue
		}
		return true
	}
	return false
}

// censusScope is what one pass over the module concluded, separated from how it is
// reported so the wording of each failure can be pinned by a unit test. The
// misdiagnosis this card fixes was a wording bug over correct data, and wording is
// only testable if it is produced by a function.
type censusScope struct {
	scopedFiles          int
	checkedDecls         int
	exercised            map[string]int
	canonicalInScope     int
	canonicalInScopedSrc int
	canonicalOutOfScope  map[string]int
	violations           map[string]bool
}

func (c censusScope) canonicalOutOfScopeTotal() int {
	total := 0
	for _, n := range c.canonicalOutOfScope {
		total += n
	}
	return total
}

// censusSanityFailures returns every self-check failure for a completed pass. The
// two canonical-count checks are deliberately separate: they have different causes
// and different remedies, and one counter serving both is what made a file leaving
// scope read as a broken parser.
func censusSanityFailures(c censusScope) []string {
	var failures []string
	if c.scopedFiles == 0 {
		failures = append(failures, "no file in the module obtains an isolated node handle — the census is checking nothing")
	}
	if c.checkedDecls == 0 {
		failures = append(failures, "no callFunctionOn declaration found in the files that obtain isolated handles — the census is checking nothing")
	}
	// Per ambient global, not in total: one of them falling to zero mentions is how
	// a whole half of the rule stops being scanned while the census stays green.
	for _, ambient := range sortedKeys(ambientShadows) {
		if c.exercised[ambient] == 0 {
			failures = append(failures, fmt.Sprintf("no declaration mentions the ambient %s any more; drop it from ambientShadows so the recorded rule stays true", ambient))
		}
	}
	// A file carrying the preamble that matches no isolatedHandleTokens entry has
	// not changed shape — it has left scope, and every declaration in it stopped
	// being scanned. The remedy is the producer list, not the parser.
	for _, name := range sortedKeys(c.canonicalOutOfScope) {
		failures = append(failures, fmt.Sprintf("%s carries the canonical preamble %d time(s) but matches no entry in isolatedHandleTokens, so none of its callFunctionOn declarations are scanned; it shadows correctly and is simply out of scope — add the name it now obtains its isolated handle through to isolatedHandleTokens",
			name, c.canonicalOutOfScope[name]))
	}
	// Only for files the census did scan: the preamble turning up in a declaration
	// the walk never reached would mean a literal it cannot see.
	if c.canonicalInScope != c.canonicalInScopedSrc {
		failures = append(failures, fmt.Sprintf("the canonical preamble appears %d times in scoped files but only %d were inside a declaration this census recognises; a declaration is assembled in a shape the scan cannot see",
			c.canonicalInScopedSrc, c.canonicalInScope))
	}
	for _, name := range sortedKeys(ambientGlobalsExceptions) {
		if !c.violations[name] {
			failures = append(failures, fmt.Sprintf("%s no longer reads an ambient global unshadowed (%s); remove it from ambientGlobalsExceptions so the recorded exceptions stay true", name, ambientGlobalsExceptions[name]))
		}
	}
	return failures
}

// ambientRead is one ambient global mentioned by one callFunctionOn declaration,
// shadowed or not: the counters need the shadowed ones and the failures need the rest.
type ambientRead struct {
	file      string
	line      int
	ambient   string
	offending string
	forms     []shadowForm
	shadowed  bool
}

func ambientReads(name string, decls []jsLiteral) []ambientRead {
	var reads []ambientRead
	for _, decl := range decls {
		body := jsCode(decl.text)
		for _, ambient := range sortedKeys(ambientShadows) {
			at := ambientMention(body, ambient)
			if at < 0 {
				continue
			}
			reads = append(reads, ambientRead{
				file:      name,
				line:      decl.lineAt(at),
				ambient:   ambient,
				offending: lineAtOffset(body, at),
				forms:     ambientShadows[ambient],
				shadowed:  hasShadow(body, ambientShadows[ambient]),
			})
		}
	}
	return reads
}

// ambientViolationFailure is the wording of the census's own failure, a function so a
// test can pin what the reader is told — the same reason censusSanityFailures is one.
func ambientViolationFailure(read ambientRead) string {
	return fmt.Sprintf("%s:%d reads the ambient %s inside a callFunctionOn declaration without deriving it from the node:\n  %s\n"+
		"An isolated handle runs in the TOP frame's world, so the ambient %s is not the node's — frame offsets vanish and hit tests run on the wrong document.\n"+
		"Add one of:\n  %s",
		read.file, read.line, read.ambient, read.offending, read.ambient,
		strings.Join(formLines(read.forms), "\n  "))
}

func sortedKeys[V any](m map[string]V) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

func TestIsolatedHandleDeclarationsShadowAmbientGlobals(t *testing.T) {
	scope := censusScope{
		exercised:           map[string]int{},
		canonicalOutOfScope: map[string]int{},
		violations:          map[string]bool{},
	}

	for _, file := range srccensus.Tree(t, "../..", moduleGoFileFloor) {
		name, src := file.Name, file.Text
		canonical := strings.Count(jsCode(src), canonicalViewLine)

		inScope := false
		for _, token := range isolatedHandleTokens {
			if strings.Contains(src, token) {
				inScope = true
				break
			}
		}
		if !inScope {
			// Counted per file and reported as a scope problem, not folded into the
			// in-scope total: a preamble here means the file left scope, which the
			// single counter used to report as a parser blind spot.
			if canonical > 0 {
				scope.canonicalOutOfScope[name] = canonical
			}
			continue
		}
		scope.scopedFiles++
		scope.canonicalInScopedSrc += canonical

		literals, parseErr := goRawLiterals(name, src)
		if parseErr != nil {
			t.Errorf("%s: parse: %v", name, parseErr)
			continue
		}
		decls := callFunctionDeclarations(literals)
		for _, decl := range decls {
			scope.checkedDecls++
			scope.canonicalInScope += strings.Count(jsCode(decl.text), canonicalViewLine)
		}
		for _, read := range ambientReads(name, decls) {
			scope.exercised[read.ambient]++
			if read.shadowed {
				continue
			}
			scope.violations[name] = true
			if why, excused := ambientGlobalsExceptions[name]; excused {
				t.Logf("%s:%d reads ambient %s under a recorded exception (%s)", name, read.line, read.ambient, why)
				continue
			}
			t.Error(ambientViolationFailure(read))
		}
	}

	t.Logf("census scope: %d files obtain isolated handles, %d callFunctionOn declarations scanned, ambient mentions %v, canonical preamble %d/%d in scope, %d out of scope",
		scope.scopedFiles, scope.checkedDecls, scope.exercised, scope.canonicalInScope, scope.canonicalInScopedSrc, scope.canonicalOutOfScopeTotal())
	for _, failure := range censusSanityFailures(scope) {
		t.Error(failure)
	}
}

func formLines(forms []shadowForm) []string {
	lines := make([]string, 0, len(forms))
	for _, form := range forms {
		if form.requires != "" {
			lines = append(lines, form.line+"   (only with "+form.requires+")")
			continue
		}
		lines = append(lines, form.line)
	}
	return lines
}

// The misdiagnosis this card fixes: a file that keeps a correct preamble and merely
// stops matching isolatedHandleTokens used to fail claiming a declaration was
// assembled in a shape the scan cannot see. The parser was working; the file had
// left scope, taking every declaration in it out of the census. Sending the reader
// to audit the parser hides the real remedy, which is the producer list.
//
// Driven through censusSanityFailures rather than the module walk, because the
// defect is the WORDING over correct data — and wording is only pinnable if the
// data can be stated directly.
func TestALeftScopeFileIsReportedAsScopeNotAsDeclarationAssembly(t *testing.T) {
	scope := censusScope{
		scopedFiles:          8,
		checkedDecls:         8,
		exercised:            map[string]int{"window": 3, "document": 2},
		canonicalInScope:     3,
		canonicalInScopedSrc: 3,
		canonicalOutOfScope:  map[string]int{"internal/cdptk/screenshot.go": 1},
		violations:           map[string]bool{},
	}

	failures := censusSanityFailures(scope)
	if len(failures) != 1 {
		t.Fatalf("censusSanityFailures = %d failures, want exactly the scope one:\n  %s", len(failures), strings.Join(failures, "\n  "))
	}
	got := failures[0]

	for _, want := range []string{"internal/cdptk/screenshot.go", "isolatedHandleTokens", "out of scope", "shadows correctly"} {
		if !strings.Contains(got, want) {
			t.Errorf("failure does not mention %q, so the reader is not pointed at the scope list:\n  %s", want, got)
		}
	}
	if strings.Contains(got, "shape the scan cannot see") {
		t.Errorf("a file leaving scope is still reported as a parser blind spot:\n  %s", got)
	}
}

// The other half of the split must keep its own wording: a preamble inside a scoped
// file that no recognised declaration contains really is a parser blind spot.
func TestAnUnreachedInScopePreambleIsStillReportedAsDeclarationAssembly(t *testing.T) {
	scope := censusScope{
		scopedFiles:          9,
		checkedDecls:         9,
		exercised:            map[string]int{"window": 4, "document": 2},
		canonicalInScope:     3,
		canonicalInScopedSrc: 4,
		canonicalOutOfScope:  map[string]int{},
		violations:           map[string]bool{},
	}

	failures := censusSanityFailures(scope)
	if len(failures) != 1 {
		t.Fatalf("censusSanityFailures = %d failures, want exactly the blind-spot one:\n  %s", len(failures), strings.Join(failures, "\n  "))
	}
	if got := failures[0]; !strings.Contains(got, "shape the scan cannot see") {
		t.Errorf("an unreached in-scope preamble is no longer reported as declaration assembly:\n  %s", got)
	}
}

// Both conditions at once must raise both failures — the point of splitting them is
// that neither masks the other.
func TestScopeLossAndBlindSpotAreReportedSeparately(t *testing.T) {
	scope := censusScope{
		scopedFiles:          8,
		checkedDecls:         8,
		exercised:            map[string]int{"window": 3, "document": 2},
		canonicalInScope:     2,
		canonicalInScopedSrc: 3,
		canonicalOutOfScope:  map[string]int{"internal/cdptk/annotate.go": 1},
		violations:           map[string]bool{},
	}

	failures := censusSanityFailures(scope)
	if len(failures) != 2 {
		t.Fatalf("censusSanityFailures = %d failures, want both:\n  %s", len(failures), strings.Join(failures, "\n  "))
	}
	joined := strings.Join(failures, "\n")
	if !strings.Contains(joined, "isolatedHandleTokens") || !strings.Contains(joined, "shape the scan cannot see") {
		t.Errorf("one condition masked the other:\n  %s", joined)
	}
}

// The probe from the card, as source rather than as an edit to a real file: a chain
// whose own operands are raw literals, with a THIRD raw literal inside a call operand
// the chain cannot absorb. The walk used to consume the chain and return false, so the
// nested literal — a callFunctionOn declaration in its own right, reading an
// unshadowed ambient window — was recorded nowhere and the census stayed green.
const nestedDeclarationProbe = "package p\n\n" +
	"import \"strings\"\n\n" +
	"var probeWrapped = `function(){` + strings.TrimSpace(`function(){ return window.innerWidth; }`) + `}`\n"

func TestTheWalkReachesARawLiteralInsideAnUnabsorbedChainOperand(t *testing.T) {
	literals, err := goRawLiterals("probe.go", nestedDeclarationProbe)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, lit := range literals {
		if strings.Contains(lit.text, "window.innerWidth") {
			found = true
		}
	}
	if !found {
		t.Fatalf("the walk recorded %d literal(s), none holding the literal inside the call operand: a chain consumed whole hides every literal its operands were not able to absorb, and this census fails open when a literal is recorded nowhere", len(literals))
	}
}

func TestTheNestedDeclarationProbeIsReportedWithItsFileLineAndAmbientRead(t *testing.T) {
	literals, err := goRawLiterals("internal/cdptk/screenshot.go", nestedDeclarationProbe)
	if err != nil {
		t.Fatal(err)
	}

	var failures []string
	for _, read := range ambientReads("internal/cdptk/screenshot.go", callFunctionDeclarations(literals)) {
		if read.shadowed {
			continue
		}
		failures = append(failures, ambientViolationFailure(read))
	}

	if len(failures) != 1 {
		t.Fatalf("the probe produced %d violation(s), want exactly one:\n  %s", len(failures), strings.Join(failures, "\n  "))
	}
	// Line 5 is where the nested literal sits in the probe source. Pinning the line and
	// not just the file is the difference between a report a reader can act on and one
	// that sends them to audit a whole file.
	for _, want := range []string{"internal/cdptk/screenshot.go:5", "window", "window.innerWidth"} {
		if !strings.Contains(failures[0], want) {
			t.Errorf("the failure does not name %q, so the reader cannot find the read it describes:\n  %s", want, failures[0])
		}
	}
}

// The card fixes ONE operand shape. This fixes the property, whatever shape the next
// operand takes: every raw literal in every module file must be reached by the census
// walk. Without it the fail-closed guarantee is prose — and prose is what let a walk
// that silently stopped descending pass for a whole release.
func TestTheCensusWalkReachesEveryRawLiteralInTheModule(t *testing.T) {
	total := 0
	for _, file := range srccensus.Tree(t, "../..", moduleGoFileFloor) {
		_, reached, err := goRawLiteralsWithReach(file.Name, file.Text)
		if err != nil {
			t.Errorf("%s: parse: %v", file.Name, err)
			continue
		}
		every, err := everyRawLiteralOffset(file.Name, file.Text)
		if err != nil {
			t.Errorf("%s: parse: %v", file.Name, err)
			continue
		}
		total += len(every)

		for _, offset := range sortedInts(every) {
			if reached[offset] {
				continue
			}
			t.Errorf("%s: the raw literal at byte offset %d is in the file but the census walk never took it in, so JS assembled there is unguarded; the walk consumed some enclosing expression whole without descending into the part holding this literal", file.Name, offset)
		}
	}

	if total < 2000 {
		t.Fatalf("the naive census found only %d raw literals in the whole module; it has stopped seeing the tree and this sweep would pass vacuously", total)
	}
	t.Logf("walk reach: %d raw literals in the module, 0 missed", total)
}

// everyRawLiteralOffset is the naive oracle: every raw literal in the file, with no
// notion of chains. Independent of the census walk on purpose — an oracle sharing the
// walk's descent decisions could not detect a wrong one.
func everyRawLiteralOffset(name, src string) (map[int]bool, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, name, src, parser.ParseComments)
	if err != nil {
		return nil, err
	}

	offsets := map[int]bool{}
	ast.Inspect(file, func(n ast.Node) bool {
		lit, ok := n.(*ast.BasicLit)
		if !ok {
			return true
		}
		if _, raw := rawLiteralText(lit); raw {
			offsets[fset.Position(lit.Pos()).Offset] = true
		}
		return true
	})
	return offsets, nil
}

func sortedInts(m map[int]bool) []int {
	out := make([]int, 0, len(m))
	for k := range m {
		out = append(out, k)
	}
	sort.Ints(out)
	return out
}
