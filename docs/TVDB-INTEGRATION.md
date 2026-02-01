# TVDB ID Integration - Implementation Summary

## Overview

Added TVDB ID support to the Stalkeer application to improve matching between Sonarr/Radarr items and the local database. This enables more accurate matching as Sonarr provides TVDB IDs natively.

## Changes Made

### 1. Database Schema Updates

**Models Updated:**
- [internal/models/filter_config.go](internal/models/filter_config.go) - Added `TVDBID` field to `Movie` struct
- [internal/models/processing_log.go](internal/models/processing_log.go) - Added `TVDBID` field to `TVShow` struct

**Fields Added:**
```go
// Movie
TVDBID *int `gorm:"index:idx_movies_tvdb" json:"tvdb_id,omitempty"`

// TVShow
TVDBID *int `gorm:"index:idx_tvshows_tvdb" json:"tvdb_id,omitempty"`
```

**Database Changes:**
- New column `tvdb_id` in `movies` table (nullable integer)
- New column `tvdb_id` in `tvshows` table (nullable integer)
- New indexes: `idx_movies_tvdb` and `idx_tvshows_tvdb`

### 2. TMDB Client Enhancements

**File:** [internal/external/tmdb/tmdb.go](internal/external/tmdb/tmdb.go)

**New Types:**
```go
type ExternalIDs struct {
    IMDBID      *string `json:"imdb_id"`
    TVDBID      *int    `json:"tvdb_id"`
    FacebookID  *string `json:"facebook_id"`
    InstagramID *string `json:"instagram_id"`
    TwitterID   *string `json:"twitter_id"`
}
```

**New Methods:**
- `GetMovieExternalIDs(movieID int) (*ExternalIDs, error)` - Fetches external IDs for movies from TMDB
- `GetTVShowExternalIDs(tvShowID int) (*ExternalIDs, error)` - Fetches external IDs for TV shows from TMDB

### 3. Processor Updates

**File:** [internal/processor/processor.go](internal/processor/processor.go)

**enrichMovie Changes:**
- Now calls `GetMovieExternalIDs()` after fetching movie details
- Stores TVDB ID when creating new movie records
- Updates existing movie records with TVDB ID if missing

**enrichTVShow Changes:**
- Now calls `GetTVShowExternalIDs()` after fetching TV show details
- Stores TVDB ID when creating new TV show records
- Updates existing TV show records with TVDB ID if missing

**Error Handling:**
- External IDs fetch failures are logged as warnings but don't fail the enrichment process
- TVDB ID is optional and gracefully handled when missing

### 4. Matcher Enhancements

**File:** [internal/matcher/matcher.go](internal/matcher/matcher.go)

**New Functions:**
```go
// Primary matching using TVDB ID with TMDB ID fallback
MatchMovieByTVDB(db, tvdbID, tmdbID, title, year) (*Movie, *ProcessedLine, int, error)
MatchTVShowByTVDB(db, tvdbID, tmdbID, title, season, episode) (*TVShow, *ProcessedLine, int, error)
```

**Matching Strategy:**
1. **First Priority:** Exact TVDB ID match (100% confidence)
2. **Second Priority:** Exact TMDB ID match (100% confidence)
3. **Fallback:** Fuzzy title/year matching (variable confidence)

### 5. Command Integration

**File:** [cmd/main.go](cmd/main.go)

**Radarr Command:**
- Updated to use `MatchMovieByTVDB()` instead of `MatchMovieByTMDB()`
- Passes TVDB ID as 0 (Radarr doesn't provide it directly)
- Relies on TMDB ID and database TVDB storage

**Sonarr Command:**
- Updated to use `MatchTVShowByTVDB()` instead of `MatchTVShowByTMDB()`
- Passes `series.TvdbID` from Sonarr API
- Improved matching accuracy for TV episodes

## Benefits

1. **Improved Matching Accuracy:**
   - Sonarr natively provides TVDB IDs, enabling exact matches
   - Reduces false positives in matching

2. **Better Data Consistency:**
   - TVDB IDs stored from TMDB external_ids endpoint
   - Consistent identifier across different metadata sources

3. **Flexible Matching:**
   - Multiple matching strategies (TVDB → TMDB → fuzzy)
   - Graceful degradation when IDs are unavailable

4. **Backward Compatibility:**
   - TVDB ID is optional (nullable)
   - Existing records still work with TMDB ID matching
   - Automatic backfilling of TVDB IDs for existing records

## Migration Path

When the application runs:
1. New items will have TVDB IDs populated automatically
2. Existing items without TVDB IDs will continue to match via TMDB ID
3. When existing items are re-enriched, TVDB IDs will be added
4. No manual migration required

## API Calls

**New TMDB API Endpoints Used:**
- `GET /movie/{movie_id}/external_ids` - For movies
- `GET /tv/{tv_id}/external_ids` - For TV shows

**Rate Limiting:**
- External IDs calls use the same retry/circuit breaker mechanisms
- Failures are logged but don't block enrichment

## Testing

Build verification completed successfully:
```bash
go build ./...  # Success
```

Test status:
- Models tests: ✅ Pass
- TMDB client: ⚠️ Requires API key (expected)
- Matcher: ⚠️ Test database schema issue (pre-existing)
- Processor: ⚠️ Config validation (pre-existing)

## Documentation Updates

- [docs/DATABASE.md](docs/DATABASE.md) - Added TVDB ID columns to schema documentation
- Added indexes documentation
- Updated table descriptions

## Future Enhancements

1. **Backfill Script:** Create a command to backfill TVDB IDs for existing records
2. **Statistics:** Track matching success rates by ID type (TVDB vs TMDB vs fuzzy)
3. **API Response:** Include TVDB ID in API responses for client use

## Files Modified

1. `internal/models/filter_config.go` - Movie model
2. `internal/models/processing_log.go` - TVShow model
3. `internal/external/tmdb/tmdb.go` - TMDB client
4. `internal/processor/processor.go` - Enrichment logic
5. `internal/matcher/matcher.go` - Matching logic
6. `cmd/main.go` - Command integration
7. `docs/DATABASE.md` - Documentation

## Breaking Changes

None. All changes are additive and backward compatible.
