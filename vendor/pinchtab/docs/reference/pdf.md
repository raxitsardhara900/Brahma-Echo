# PDF

Render the current page as a PDF.

```bash
curl "http://localhost:9867/pdf?output=file"
# Response: {"path":"/path/to/state/pdfs/page-20260308-120001.pdf","size":48210}

# CLI Alternative (human-readable by default)
pinchtab pdf -o page.pdf
# Output: Saved page.pdf (48210 bytes)

pinchtab pdf                        # Auto-generates filename: page-20260308-120001.pdf
```

## CLI Flags

| Flag | Description |
|------|-------------|
| `-o`, `--output` | Save PDF to file path |
| `--landscape` | Landscape orientation |
| `--scale` | Page scale (e.g. 0.5) |
| `--paper-width` | Paper width (inches) |
| `--paper-height` | Paper height (inches) |
| `--page-ranges` | Page ranges (e.g. 1-3) |
| `--prefer-css-page-size` | Use CSS page size |
| `--display-header-footer` | Show header/footer |
| `--header-template` | Header HTML template |
| `--footer-template` | Footer HTML template |
| `--margin-*` | Margins (top, bottom, left, right) |
| `--generate-tagged-pdf` | Generate tagged PDF |
| `--generate-document-outline` | Generate document outline |
| `--tab` | Target specific tab |

## API Parameters

| Parameter | Description |
|-----------|-------------|
| `output` | `file` to save server-side |
| `raw` | `true` for raw PDF bytes |
| `landscape` | Landscape orientation |
| `scale` | Page scale |
| `paperWidth`, `paperHeight` | Paper dimensions |

## Related Pages

- [Text](./text.md)
- [Screenshot](./screenshot.md)
