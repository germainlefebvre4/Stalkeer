# TMDB Integration - Task Update Summary

**Date**: January 29, 2026  
**Update Type**: New Task Addition  
**Task ID**: 2.7

## Overview

Added comprehensive TMDB (The Movie Database) integration task to enrich parsed M3U playlist items with authoritative metadata.

## New Task Created

### Task 2.7: TMDB Integration & Content Enrichment

**Location**: [.github/plans/tasks/2.7-tmdb-integration.md](.github/plans/tasks/2.7-tmdb-integration.md)  
**Phase**: 2 (Core Functionality)  
**Complexity**: Large  
**Estimated Timeline**: 9-13 days (approximately 2 weeks)

## What This Task Delivers

### 1. TMDB API Client
- Search for movies and TV shows by title and year
- Retrieve detailed metadata including:
  - Titles (original and localized)
  - Release/air dates
  - Posters and images
  - Plot overviews
  - Genres
  - Runtime/episode information
  - Popularity scores
- Rate limiting (40 requests per 10 seconds)
- Retry logic with exponential backoff
- Circuit breaker pattern
- Response caching (24-hour TTL)

### 2. Content Enrichment Service
- Intelligent matching algorithm:
  - Clean title extraction from M3U data
  - Year parsing and matching
  - Fuzzy matching with Levenshtein distance
  - Popularity-based tie-breaking
- Batch processing with progress tracking
- Deduplication of TMDB records
- Resume capability after interruption

### 3. Data Storage
- Populate `movies` table with TMDB metadata:
  - tmdb_id, tmdb_title, tmdb_year
  - tmdb_genres (JSON), duration
- Populate `tvshows` table with TMDB metadata:
  - tmdb_id, tmdb_title, tmdb_year
  - tmdb_genres (JSON), season, episode
- Link enriched data to `processed_lines` via foreign keys

### 4. CLI & API Integration
- CLI command: `stalkeer enrich [--content-type movies|tvshows] [--dry-run] [--force]`
- REST API endpoints:
  - `POST /api/v1/enrich` - Trigger enrichment
  - `GET /api/v1/enrich/status` - Monitor progress

### 5. Configuration
```yaml
tmdb:
  api_key: "your_tmdb_api_key_here"
  base_url: "https://api.themoviedb.org/3"
  timeout: 10s
  rate_limit:
    requests_per_10s: 40
  cache:
    enabled: true
    ttl: 24h
    max_entries: 10000
```

## Task Dependencies

This task depends on:
- ✅ Task 1.3: Database Schema (Movies and TVShows tables)
- ⏳ Task 2.1: M3U Parser (items must be parsed first)
- ⏳ Task 2.2: Content Classification (content type detection)

This task is a dependency for:
- Task 4.1: Download Implementation (enhanced matching)

## Changes to Existing Tasks

### Updated: Task 4.1 - Download Implementation
- Changed TMDB reference from "optional" to required dependency
- Updated technical approach to reference Task 2.7
- Added Task 2.7 to dependencies section

### Updated: docs/STATUS.md
- Added Task 2.7 to Phase 2 task list
- Updated phase descriptions to match actual task structure
- Clarified task organization

## Implementation Highlights

### Error Handling
Comprehensive error handling for:
- Missing API key
- API authentication failures (401)
- Not found responses (404)
- Rate limiting (429) with Retry-After header support
- Network timeouts with retry
- JSON parsing errors
- No matches found
- Multiple ambiguous matches

### Performance Considerations
- In-memory LRU cache (max 10,000 entries)
- Batch processing (default 50 items)
- Rate limit compliance
- Circuit breaker prevents cascading failures
- Performance target: 1000 items in <5 minutes

### Testing Strategy
- Unit tests with mocked TMDB responses
- Integration tests with real TMDB API (test key)
- Performance benchmarks
- Error scenario coverage
- Matching algorithm accuracy tests

## Next Steps

1. **Immediate**: Review and approve task specification
2. **Setup**: Obtain TMDB API key (free at https://www.themoviedb.org/settings/api)
3. **Implementation**: Follow 6-step implementation plan in task file
4. **Integration**: Coordinate with Tasks 2.1 and 2.2 completion

## Benefits

- **Data Quality**: Authoritative metadata from TMDB
- **User Experience**: Rich information for media items (posters, descriptions)
- **Matching Accuracy**: Better Radarr/Sonarr matching with normalized titles
- **Search**: Enhanced search capabilities with genres, years
- **Deduplication**: Prevents duplicate entries across M3U sources
- **International Support**: TMDB supports multiple languages

## Notes

- TMDB API is free for non-commercial use with generous rate limits
- API key registration required but straightforward
- TMDB data is crowdsourced and highly accurate
- Poster images can be downloaded in future enhancement
- Genre mapping available from TMDB genres endpoint
- Consider monthly refresh of metadata for updates

## Files Modified

1. **Created**: [.github/plans/tasks/2.7-tmdb-integration.md](.github/plans/tasks/2.7-tmdb-integration.md)
2. **Updated**: [.github/plans/tasks/4.1-download-implementation.md](.github/plans/tasks/4.1-download-implementation.md)
3. **Updated**: [docs/STATUS.md](../../docs/STATUS.md)
4. **Created**: This summary document

---

**Author**: GitHub Copilot  
**Review Status**: Pending  
**Priority**: High (blocks Phase 4 download implementation)
