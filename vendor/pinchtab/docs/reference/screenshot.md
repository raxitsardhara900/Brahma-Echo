# Screenshot

Capture the current page as an image. The API defaults to **JPEG**. The CLI
uses PNG when `-o` ends in `.png` and JPEG otherwise; an explicit `--format`
always takes precedence.

```bash
# Get raw PNG bytes
curl "http://localhost:9867/screenshot?format=png&raw=true" > page.png

# Capture a specific element (selector supports ref/CSS/XPath/text)
curl "http://localhost:9867/screenshot?selector=%23checkout-button&raw=true" > button.jpg

# Half-size output (quarter the pixels)
curl "http://localhost:9867/screenshot?scale=0.5&raw=true" > page-half.jpg

# Capture the entire scrollable document, not just the visible viewport
curl "http://localhost:9867/screenshot?beyondViewport=true&raw=true" > fullpage.jpg

# Get JSON with base64 JPEG (default)
curl "http://localhost:9867/screenshot"

# Save to server state directory
curl "http://localhost:9867/screenshot?output=file"
```

## Response (JSON)

```json
{
  "path": "/path/to/state/screenshots/screenshot-20260308-120001.jpg",
  "size": 34567,
  "format": "jpeg",
  "timestamp": "20260308-120001"
}
```

## Useful flags

### API Query Parameters

- `format`: `jpeg` (default) or `png`.
- `quality`: JPEG quality `0-100` (default: `80`). Ignored for PNG.
- `selector`: Unified selector to capture one element (e.g. `e5`, `#id`, `xpath://...`, `text:Submit`).
- `scale`: Rescale the output bitmap. Default `1`. `0.5` halves each axis (quarter the pixels).
- `beyondViewport`: `true` to capture the full scrollable document instead of just the visible viewport. Ignored when `selector` is set. With `annotate=true` the returned box coordinates are document-relative.
- `raw`: `true` to return image bytes directly instead of JSON.
- `output`: `file` to save to state directory.
- `tabId`: Target a specific tab.

### CLI

- `-o <path>`: Save to a specific path. A `.png` extension selects PNG when `--format` is omitted; other extensions use JPEG.
- `--format <jpeg|png>`: Explicitly select the image format, overriding extension inference.
- `-q <0-100>`: Set JPEG quality.
- `-s <selector>`: Capture a specific element.
- `--scale <f>`: Bitmap rescale (e.g. `0.5`).
- `--beyond-viewport`: Capture the full scrollable document. Ignored when `--selector` is set.
- `--tab <id>`: Target a specific tab.

## When to use `pinchtab capture` instead

`screenshot` returns image bytes only. When the model needs to act on refs
in the same turn it reads pixels, use [`capture`](./capture.md) — paired
image + accessibility snapshot from the same DOM epoch.

## Related Pages

- [Capture](./capture.md)
- [Snapshot](./snapshot.md)
- [PDF](./pdf.md)
