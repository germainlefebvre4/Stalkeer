## Context

The `download radarr` command fetches "wanted" movies from Radarr's API and attempts to match each entry against the local database using `MatchMovieByTVDB(db, tvdbID, tmdbID, title, year)`. Radarr's REST API returns `tvdbId` on every movie object, but the local `radarr.Movie` struct only parses `tmdbId` — so `tvdbID` is always `0` at call time, permanently bypassing the TVDB-primary key path and falling through to TMDB-ID matching.

By contrast, `download sonarr` already parses `TvdbID int json:"tvdbId"` in `sonarr.Series` and passes it correctly.

The `process` command already fetches TVDB IDs from TMDB's `external_ids` endpoint and persists them in `Movie.tvdb_id` and `TVShow.tvdb_id` — that side is correct.

## Goals / Non-Goals

**Goals:**
- Parse `tvdbId` from Radarr's movie API response so TVDB-primary matching works
- Pass the parsed TVDB ID to `MatchMovieByTVDB` in `download radarr`
- Bring Radarr's struct in line with the existing Sonarr pattern

**Non-Goals:**
- Changes to the `process` command (already correct)
- Changes to `MatchMovieByTVDB` logic (already correct)
- Changes to the Sonarr download flow (already correct)

## Decisions

### D1: Fix the Radarr struct, not the matcher

The bug is entirely in data parsing — Radarr sends `tvdbId` and we discard it. Adding the field to the struct is the minimal, correct fix. Changing `MatchMovieByTVDB` or adding a separate lookup would be over-engineering.

**Alternatives considered:**
- Call a second Radarr API endpoint per movie to fetch TVDB ID — unnecessary API overhead; the main listing already includes it.
- Use only TMDB-ID matching in the radarr command — this already works but is less reliable (falls back to fuzzy title matching when TMDB-ID lookup fails).

### D2: Matching priority stays TVDB → TMDB → fuzzy title

`MatchMovieByTVDB` already implements this cascade. With a real TVDB ID available, primary matches will be exact key lookups with 100% confidence.

## Risks / Trade-offs

- [Risk] Radarr returns `tvdbId: 0` for movies without a TVDB entry → Mitigation: `MatchMovieByTVDB` already guards with `if tvdbID > 0`; zero is harmlessly skipped.
- [Risk] `tvdbId` field absent from older Radarr API versions → Mitigation: Go's JSON decoder leaves the field as zero-value (0) when absent — safe.

## Migration Plan

No data migration or deployment steps required. The struct change is backwards-compatible: if Radarr doesn't return `tvdbId`, the field deserialises to `0` and behavior is identical to today.
