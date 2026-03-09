## Context

M3U playlists routinely contain multiple entries for the same movie or TV episode — same TMDB identity, different URLs, often different quality labels (720p, 1080p, 4K, SD, etc.). Today the system deduplicates at the `(tvgName + URL)` hash level, so these entries correctly produce distinct `ProcessedLine` rows all pointing to the same `Movie`/`TVShow`. However:

1. `ProcessedLine` has no `resolution` column — the quality detected by the classifier is discarded after classification.
2. `matcher.MatchMovieByTMDB()` returns a single `ProcessedLine` sorted only by `created_at DESC` — no quality awareness.
3. Download commands make one attempt with that single URL; failure is terminal for that content item.
4. `ExtractResolution()` in the classifier uses `strings.Contains()` despite precompiled word-boundary regex patterns (`c.resolutionPatterns`) being present in the struct — a silent inconsistency introduced alongside `ExtractSeasonEpisode()`.
5. `OverridesID`/`OverridesAt`/`Overrides` are dead fields: defined in the model, never written or read in application code.

## Goals / Non-Goals

**Goals:**
- Persist resolution per `ProcessedLine` so quality is queryable.
- Fix `ExtractResolution()` to use the existing `c.resolutionPatterns` (word-boundary, case-insensitive regex).
- Select download URL with quality preference: `720p → 1080p → 4K → 480p → nil`.
- Within the same quality tier, prefer the most recently added entry (`created_at DESC`).
- Implement a fallback loop in download commands: try candidates in order, mark each failed one, move on.
- Remove dead `OverridesID`/`OverridesAt`/`Overrides` fields cleanly.

**Non-Goals:**
- Per-user or per-content quality configuration (global preference only).
- Changing `resume-downloads` behavior — it retries the same `ProcessedLine` it originally downloaded; quality re-selection at resume time is out of scope.
- Retroactively populating `resolution` for existing rows (they stay `NULL` and sort last in preference order, which is the correct fallback).
- Modifying the parser or hash function.

## Decisions

### D1: Persist resolution on `ProcessedLine`, not on `Movie`/`TVShow`

Resolution is a property of a specific M3U stream entry, not of the content itself. Multiple entries for the same movie can have different resolutions. Storing it on `ProcessedLine` preserves this correctly and avoids denormalization on the content entities.

_Alternative_: Store preferred resolution on `Movie`. Rejected — loses per-entry quality info and requires update logic on every re-parse.

### D2: Quality preference order: `720p → 1080p → 4K → 480p → nil`

The goal is the "sweet spot" quality — good enough image, reasonable file size — rather than maximum quality. The fallback goes upward (larger) before downward (smaller), then `nil` (unknown) last.

Implemented as a `CASE` expression in SQL ORDER BY, not application-side sorting, to keep the query efficient and atomic.

### D3: New `FindMovieDownloadCandidates()` / `FindTVShowDownloadCandidates()` functions in `matcher`

Rather than changing the signature of `MatchMovieByTMDB()` (breaking), add dedicated functions that return `[]ProcessedLine` ordered by quality preference + recency. The download commands call the existing match function to find the content entity, then call the candidates function to get the ordered URL list.

_Alternative_: Change `MatchMovieByTMDB()` return signature. Rejected — breaks the matcher contract; matching and candidate selection are different concerns.

### D4: Fallback loop is in `cmd/main.go`, not in the downloader

The `Downloader.Download()` interface works on a single URL + `ProcessedLineID`. URL selection and retry-with-alternative are the responsibility of the caller (the command), not the downloader. This keeps the downloader single-purpose and keeps the DB state updates (marking a line as `failed`) clearly owned by the command layer.

### D5: `OverridesID`/`OverridesAt`/`Overrides` are dropped, not repurposed

The fields have no semantic definition in any active code path. Repurposing them for a new concern would be confusing. A clean drop with migration is safer.

## Risks / Trade-offs

- **Existing `ProcessedLine` rows have `resolution = NULL`**: They sort last in candidate selection (preference rank 5). This is correct — unknown quality is last resort. No data migration needed.
- **`strings.Contains("hd")` fix changes classification for existing unparsed entries**: After the fix, entries that previously incorrectly matched `hd` (e.g., a title containing "shade") will no longer be assigned `720p`. The impact is limited to newly processed or force-reprocessed entries.
- **Dropping `OverridesID`**: If any external tooling or manual SQL queries reference this column, they will break. Risk is low — the column was never populated.

## Migration Plan

1. Add `ADD COLUMN resolution VARCHAR(10)` — backward compatible, `NULL` for existing rows.
2. Drop `overrides_id` and `overrides_at` columns — verify no application code references them first (confirmed: only model definition).
3. No data backfill required for `resolution`. Existing rows sort last in candidate selection, which is the correct fallback behavior.
4. Rollback: re-add dropped columns as nullable with no data loss; remove `resolution` column.
