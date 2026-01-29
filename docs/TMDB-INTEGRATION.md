# TMDB Integration Guide

## Overview

Stalkeer now includes The Movie Database (TMDB) integration to enrich media metadata during M3U playlist processing. This feature automatically fetches additional information like titles, years, genres, posters, and other metadata for movies and TV shows.

## Features

- Automatic metadata enrichment for movies and TV shows
- Multi-language support (English, French, Spanish, etc.)
- Rate limiting and circuit breaker protection
- Retry logic for failed requests
- Configurable via config file or CLI flags
- Optional skip for faster processing without enrichment

## Configuration

### API Key Setup

1. Create a free account at [https://www.themoviedb.org](https://www.themoviedb.org)
2. Navigate to Settings → API
3. Request an API key (v3 auth)
4. Copy your API key

### config.yml

Add the following configuration to your `config.yml`:

```yaml
tmdb:
  enabled: true
  api_key: your_tmdb_api_key_here  # Replace with your actual API key
  language: en-US  # Default language for metadata
```

### Environment Variables

You can also configure TMDB via environment variables:

```bash
export STALKEER_TMDB_ENABLED=true
export STALKEER_TMDB_API_KEY=your_api_key_here
export STALKEER_TMDB_LANGUAGE=en-US
```

## Usage

### Basic Processing with TMDB

```bash
# Process M3U file with TMDB enrichment (default)
stalkeer process playlist.m3u

# Process with French metadata
stalkeer process playlist.m3u --tmdb-language fr-FR

# Process with Spanish metadata
stalkeer process playlist.m3u --tmdb-language es-ES
```

### Processing Without TMDB

```bash
# Skip TMDB enrichment for faster processing
stalkeer process playlist.m3u --skip-tmdb
```

### Example Output

```
Processing M3U file: playlist.m3u

=== Processing Complete ===
Total lines in file:  1000
Successfully processed: 985
Duplicates skipped:   5
Filtered out:         8
Errors:               2

Content breakdown:
  Movies:        742
  TV Shows:      231
  Channels:      0
  Uncategorized: 12

TMDB Enrichment:
  Matched:       891
  Not found:     76
  Errors:        6
  Match rate:    92.1%

Processing time: 1.2s
```

## How It Works

### Movie Matching

1. Extract title and year from M3U entry (e.g., "The Matrix (1999)")
2. Search TMDB API for matching movie
3. Fetch detailed information (title, year, poster, overview, genres)
4. Store in `movies` table with TMDB metadata

### TV Show Matching

1. Extract show title, season, and episode from M3U entry
2. Search TMDB API for matching TV show
3. Fetch detailed information (name, year, poster, overview, genres)
4. Store in `tvshows` table with season/episode info and TMDB metadata

### Error Handling

- **Not Found**: If no match is found, the item is still saved but without TMDB metadata
- **Rate Limiting**: Circuit breaker prevents excessive requests after failures
- **Network Errors**: Automatic retry with exponential backoff
- **API Errors**: Logged as warnings but don't stop processing

## Database Schema

### movies Table

The `movies` table stores TMDB-enriched movie metadata:

```sql
CREATE TABLE movies (
    id SERIAL PRIMARY KEY,
    tmdb_id INTEGER NOT NULL,
    tmdb_title VARCHAR(255) NOT NULL,
    tmdb_year INTEGER NOT NULL,
    tmdb_genres TEXT,
    duration INTEGER,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);
```

### tvshows Table

The `tvshows` table stores TMDB-enriched TV show metadata:

```sql
CREATE TABLE tvshows (
    id SERIAL PRIMARY KEY,
    tmdb_id INTEGER NOT NULL,
    tmdb_title VARCHAR(255) NOT NULL,
    tmdb_year INTEGER NOT NULL,
    tmdb_genres TEXT,
    season INTEGER,
    episode INTEGER,
    created_at TIMESTAMP NOT NULL,
    updated_at TIMESTAMP NOT NULL
);
```

## Supported Languages

Common language codes for TMDB:

- `en-US` - English (United States)
- `fr-FR` - French (France)
- `es-ES` - Spanish (Spain)
- `de-DE` - German (Germany)
- `it-IT` - Italian (Italy)
- `ja-JP` - Japanese (Japan)
- `ko-KR` - Korean (South Korea)
- `pt-BR` - Portuguese (Brazil)
- `zh-CN` - Chinese (Simplified)

See [TMDB Language Documentation](https://developers.themoviedb.org/3/configuration/get-primary-translations) for complete list.

## Performance Considerations

### Rate Limits

TMDB API has rate limits:
- 40 requests per 10 seconds
- Circuit breaker opens after 5 consecutive failures
- Automatic retry with exponential backoff

### Processing Speed

- **With TMDB**: ~100-200 entries/minute (depends on API response time)
- **Without TMDB**: ~1000+ entries/minute

For large playlists (>10,000 entries), consider:
1. Using `--skip-tmdb` for initial import
2. Running a separate enrichment pass later
3. Processing in batches with `--limit` flag

## Troubleshooting

### "TMDB integration disabled or API key not configured"

**Solution**: Check that your API key is correctly set in `config.yml` or environment variables.

### "TMDB API rate limit exceeded"

**Solution**: Circuit breaker will automatically pause requests. Wait a minute and retry. Consider using smaller batches.

### Low match rates

**Possible causes**:
- Inconsistent naming in M3U file
- Non-English titles without proper year
- Titles with extra quality tags (FHD, 4K, etc.)

**Solutions**:
- Ensure titles include year when possible: "Movie (2024)"
- Remove excessive quality markers from source M3U
- Use manual correction for important mismatches

### "No results found for movie/TV show"

**Solution**: The title couldn't be matched in TMDB. The entry is still saved but without TMDB metadata. You can manually add metadata later via API or database update.

## API Reference

### Internal TMDB Client

The TMDB client is located at `internal/external/tmdb/` and provides:

- `SearchMovie(title string, year *int)` - Search for movies
- `SearchTVShow(title string)` - Search for TV shows
- `GetMovieDetails(movieID int)` - Get detailed movie info
- `GetTVShowDetails(tvShowID int)` - Get detailed TV show info

### Example Usage

```go
import "github.com/glefebvre/stalkeer/internal/external/tmdb"

// Create client
client := tmdb.NewClient(tmdb.Config{
    APIKey:   "your_api_key",
    Language: "en-US",
})

// Search for a movie
year := 1999
result, err := client.SearchMovie("The Matrix", &year)
if err != nil {
    // Handle error
}

// Get details
details, err := client.GetMovieDetails(result.ID)
```

## Future Enhancements

Planned features for TMDB integration:

- [ ] Caching layer to reduce API calls
- [ ] Manual match correction interface
- [ ] Batch enrichment command for existing data
- [ ] Image download and local storage
- [ ] Advanced fuzzy matching for better accuracy
- [ ] Support for TMDB v4 API

## References

- [TMDB API Documentation](https://developers.themoviedb.org/3)
- [TMDB API Terms of Use](https://www.themoviedb.org/documentation/api/terms-of-use)
- [TMDB Attribution Requirements](https://www.themoviedb.org/about/logos-attribution)
