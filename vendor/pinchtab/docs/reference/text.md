# Text

Extract text from the current page or a specific element.

By default, PinchTab runs a Readability-style extraction against the current
document. Use full/raw mode when you want `document.body.innerText` instead.

## Element Selection

Extract text from a specific element using a selector:

```bash
# Positional selector argument
pinchtab text "#article-body"
pinchtab text "text:Welcome"

# Or use --selector flag
pinchtab text --selector "#article-body"
pinchtab text -s "xpath://div[@class='content']"
```

Supported selector types: ref (`e5`), CSS (`#id`), XPath (`xpath://...`), text (`text:...`).

## Frame Scope

`/text` is frame-aware:

- `--frame <id>` or `frameId=<id>` targets a specific iframe for a one-shot read
- otherwise, `/text` inherits the tab's current frame scope from [`/frame`](./frame.md)
- if no frame is selected, `/text` reads from the top-level document

## Output Format

Default output is human-readable text. Use `--json` for structured output:

```bash
pinchtab text                           # Plain text output
pinchtab text --json                    # JSON: {"url":"...","title":"...","text":"..."}
```

## Extraction Mode

Readability is an article heuristic, so on a landing page, dashboard, or docs
index it can return one block instead of the page. `/text` measures its output
against `document.body.innerText` and, when the coverage is too low, returns the
raw text instead. The JSON envelope always says which extractor produced the
text:

| Field | Description |
|-------|-------------|
| `extraction` | `readability`, `raw` (explicitly requested), or `readability_fallback` (Readability collapsed, raw text returned) |
| `textLength` | Length of the returned text |
| `rawLength` | Length of `document.body.innerText`, so coverage is computable |

With `format=text` the body stays bare and the mode is reported in the
`X-PT-Text-Extraction` response header. `mode=raw` never runs the comparison.
`truncated` keeps its meaning — text cut by `maxChars` — and is unaffected by a
fallback. The CLI prints a one-line note on stderr when a fallback fired; stdout
stays text-only.

## Examples

```bash
# Default Readability extraction
pinchtab text

# Full page text (document.body.innerText)
pinchtab text --full
pinchtab text --raw                     # Alias of --full

# Extract text from specific element
pinchtab text "#main-content"
pinchtab text --selector ".article-body"

# One-shot iframe read by frame id
pinchtab text --frame FRAME123

# API equivalent
curl "http://localhost:9867/text?mode=raw"
curl "http://localhost:9867/text?selector=%23article-body"
curl "http://localhost:9867/text?frameId=FRAME123&format=text"
```

## Flags

| Flag | Description |
|------|-------------|
| `--selector`, `-s` | Element selector (ref/CSS/XPath/text) |
| `--frame` | Extract from specific iframe by frameId |
| `--full` | Full page innerText instead of Readability |
| `--raw` | Alias for --full |
| `--json` | Output JSON instead of plain text |
| `--tab` | Target specific tab |

## API Parameters

| Parameter | Description |
|-----------|-------------|
| `selector` | Element selector for text extraction |
| `ref` | Snapshot ref (e.g., `e5`) |
| `frameId` | Target iframe ID |
| `mode` | `raw` for innerText, default for Readability |
| `maxChars` | Truncate output |
| `format` | `text` for plain text response |

Use default mode for article-like pages. Use `--full` / `mode=raw` for UI-heavy
pages such as dashboards, SERPs, grids, pricing tables, or short log panes that
Readability may trim away.

## Related Pages

- [Snapshot](./snapshot.md)
- [Frame](./frame.md)
- [PDF](./pdf.md)
