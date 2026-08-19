# Find Architecture

This page covers the implementation details behind PinchTab's semantic `find` pipeline.

## Overview

The `find` system converts accessibility snapshot nodes into lightweight descriptors, scores them against a natural-language query, and returns the best matching `ref`.

The implementation is designed to stay:

- local
- fast
- dependency-light
- recoverable after page re-renders

## Pipeline

```text
accessibility snapshot
  -> DOM metadata enrichment
  -> element descriptors
  -> lexical matcher
  -> embedding matcher
  -> combined score
  -> best ref
  -> intent cache / recovery hooks
```

## Element Descriptors

Each accessibility node is converted into a descriptor with:

- `ref`
- `role`
- `name`
- `value`
- `label`
- `placeholder`
- `alt`
- `title`
- `testid`
- `text`
- `tag`
- `interactive`
- `parent`
- `section`
- `documentIdx`
- positional hints: `depth`, `siblingIndex`, `siblingCount`, `labelledBy`

PinchTab owns extraction of DOM-only metadata from backend node ids. The structured locator parsing and matching for `role:`, `text:`, `label:`, `placeholder:`, `alt:`, `title:`, `testid:`, `first:`, `last:`, and `nth:` forms is delegated to the external `github.com/pinchtab/semantic` Go module (a sibling package, not part of this repo). The in-repo `internal/autosolver/semantic/adapter.go` is just a thin adapter that wires the module into the autosolver.

CSS, XPath, refs, frame scoping, converting a matched ref back to a backend node, and the existing DOM-backed action `text:` selector remain PinchTab responsibilities.

## Matchers

PinchTab currently uses a combined matcher built from:

- a lexical matcher
- an embedding matcher based on a hashing embedder

Default weighting is:

```text
0.6 lexical + 0.4 embedding
```

Per-request overrides exist through `lexicalWeight` and `embeddingWeight`.

## Lexical Side

The lexical matcher focuses on exact and near-exact token overlap, including role-aware matching behavior.

Useful properties:

- strong for exact words
- easy to reason about
- good precision on explicit queries like `submit button`

## Embedding Side

The embedding matcher uses a feature-hashing approach rather than an external ML model.

Useful properties:

- catches fuzzy similarity
- handles partial and sub-word overlap better
- has no model download or network dependency

## Combined Matching

The combined matcher runs lexical and embedding scoring concurrently, merges results by element ref, and applies the weighted final score.

It also uses a lower internal threshold before the final merge so that candidates which are only strong on one side are not discarded too early.

## Snapshot Dependency

`find` depends on the same accessibility snapshot/ref-cache infrastructure used by snapshot-driven interaction.

If a cached snapshot is missing, the handler tries to refresh it automatically before giving up.

## Intent Cache And Recovery

After a successful match, PinchTab records:

- the original query
- the matched descriptor
- score/confidence metadata

This allows recovery logic to attempt a semantic re-match if a later action fails because the old ref became stale after a page update.

## Routing

`POST /tabs/{id}/find` (and the active-tab shorthand `POST /find`) is registered by the shared handler layer in `internal/handlers/handlers.go`. The orchestrator proxies these requests to the correct running instance via its routing layer; it does not own the route itself.

## Design Constraints

The current design intentionally avoids:

- external embedding services
- heavyweight model dependencies
- selector-first coupling

That keeps the system portable and fast, but it also means the quality ceiling is bounded by the in-process matcher design and the quality of the accessibility snapshot.

## Performance

Benchmarks on Intel i5-4300U @ 1.90GHz:

| Operation | Elements | Latency | Allocations |
| --- | --- | --- | --- |
| Lexical Find | 16 | ~71 us | 134 allocs |
| HashingEmbedder (single) | 1 | ~11 us | 3 allocs |
| HashingEmbedder (batch) | 16 | ~171 us | 49 allocs |
| Embedding Find | 16 | ~180 us | 98 allocs |
| **Combined Find** | **16** | **~233 us** | **263 allocs** |
| Combined Find | 100 | ~1.5 ms | 1685 allocs |
