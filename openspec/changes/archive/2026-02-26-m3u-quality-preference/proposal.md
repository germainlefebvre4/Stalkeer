## Why

M3U playlists frequently contain multiple entries for the same content (same movie or episode) with different URLs and quality levels (720p, 1080p, 4K, SD). Currently the system has no awareness of resolution: it doesn't store it, doesn't use it for URL selection, and has no fallback when a URL fails. The result is arbitrary URL selection and silent failures with no retry on alternative sources.

## What Changes

- Add `resolution` field to `ProcessedLine` to persist detected quality per M3U entry
- Fix `ExtractResolution()` in the classifier to use precompiled word-boundary regex patterns (bug: patterns are compiled but never used — `strings.Contains` is used instead, causing false positives)
- Persist `classification.Resolution` onto `ProcessedLine` during processing
- Add quality-aware candidate selection in the matcher: for a given Movie/TVShow, return all eligible `ProcessedLine` records ordered by quality preference (`720p → 1080p → 4K → 480p → nil`) then by recency (`created_at DESC`)
- Implement a download fallback loop in `download radarr` and `download sonarr` commands: try candidates in order, mark each failed attempt, move to next on failure
- **BREAKING**: Remove unused `OverridesID`, `OverridesAt`, and `Overrides` fields from `ProcessedLine` (dead code — never set or read outside the model definition)

## Capabilities

### New Capabilities

- `m3u-quality-selection`: Persist resolution metadata on each M3U entry and use quality-ordered candidate lists with fallback when selecting a download URL for a given piece of content.

### Modified Capabilities

- `m3u-title-normalization`: Resolution extraction fix — `ExtractResolution()` must use the precompiled `\b`-bounded regex patterns already present in the classifier instead of `strings.Contains`.

## Impact

- `internal/models/playlist_item.go` — add `Resolution`, drop `OverridesID`/`OverridesAt`/`Overrides`
- DB migration — `ADD COLUMN resolution VARCHAR(10)`, `DROP COLUMN overrides_id`, `DROP COLUMN overrides_at`
- `internal/classifier/classifier.go` — fix `ExtractResolution()` to use `c.resolutionPatterns`
- `internal/processor/processor.go` — assign `line.Resolution` from `classification.Resolution` in `setContentType()`
- `internal/matcher/matcher.go` — add `FindMovieDownloadCandidates()` and `FindTVShowDownloadCandidates()`
- `cmd/main.go` — replace single-match download with fallback loop for `radarr` and `sonarr` commands
