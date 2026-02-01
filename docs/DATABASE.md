# Database Schema Documentation

This document describes the database schema for the Stalkeer application.

## Overview

The schema implements a polymorphic design where M3U playlist lines are stored in `processed_lines` with relationships to content-specific tables (`movies`, `tvshows`). This approach:

- Preserves original M3U line content
- Enables TMDB integration for normalized metadata
- Supports deduplication via content hashing
- Tracks processing state and version history

## Tables

### processed_lines

Stores original M3U playlist lines with polymorphic relationships to content types.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | INTEGER | PRIMARY KEY | Unique identifier |
| `line_content` | TEXT | NOT NULL | Original M3U EXTINF line |
| `line_url` | TEXT | NULLABLE | Stream URL from M3U |
| `line_hash` | VARCHAR(64) | NOT NULL, UNIQUE | SHA-256 hash for deduplication |
| `tvg_name` | VARCHAR(255) | NOT NULL | Original TVG name from M3U |
| `group_title` | VARCHAR(255) | NOT NULL | Original group title from M3U |
| `processed_at` | TIMESTAMP | NOT NULL | Processing timestamp |
| `content_type` | VARCHAR(20) | NOT NULL | Content category (movies/tvshows/channels/uncategorized) |
| `channel_id` | INTEGER | FOREIGN KEY, NULLABLE | Reference to channels |
| `movie_id` | INTEGER | FOREIGN KEY, NULLABLE | Reference to movies |
| `tvshow_id` | INTEGER | FOREIGN KEY, NULLABLE | Reference to tvshows |
| `uncategorized_id` | INTEGER | FOREIGN KEY, NULLABLE | Reference to uncategorized |
| `download_info_id` | INTEGER | FOREIGN KEY, NULLABLE | Reference to download tracking |
| `state` | VARCHAR(50) | NOT NULL | Processing state |
| `created_at` | TIMESTAMP | NOT NULL | Record creation time |
| `updated_at` | TIMESTAMP | NOT NULL | Record update time |
| `overrides_id` | INTEGER | FOREIGN KEY, NULLABLE | Self-reference for version history |
| `overrides_at` | TIMESTAMP | NULLABLE | Override timestamp |

**Indexes:**
- `idx_processed_lines_hash` on `line_hash`
- `idx_processed_lines_content` on `(content_type, state)`
- `idx_processed_lines_m3u` on `(group_title, tvg_name)`
- `idx_processed_lines_download` on `download_info_id`

**Content Types:**
- `movies` - Movie content
- `tvshows` - TV show episodes
- `channels` - Live TV channels
- `uncategorized` - Unclassified content

**Processing States:**
- `processed` - Successfully parsed and categorized
- `pending` - Awaiting processing
- `downloading` - Currently being downloaded
- `downloaded` - Download completed
- `failed` - Processing or download failed

---

### movies

Stores movie metadata from TMDB with deduplication by title and year.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | INTEGER | PRIMARY KEY | Unique identifier |
| `tmdb_id` | INTEGER | NOT NULL | TMDB database ID |
| `tvdb_id` | INTEGER | NULLABLE | TVDB database ID (from TMDB external_ids) |
| `tmdb_title` | VARCHAR(255) | NOT NULL | Movie title from TMDB |
| `tmdb_year` | INTEGER | NOT NULL | Release year from TMDB |
| `tmdb_genres` | TEXT | NULLABLE | Genres as JSON array |
| `duration` | INTEGER | NULLABLE | Duration in minutes |
| `created_at` | TIMESTAMP | NOT NULL | Record creation time |
| `updated_at` | TIMESTAMP | NOT NULL | Record update time |

**Indexes:**
- `idx_movies_tmdb` on `tmdb_id`
- `idx_movies_tvdb` on `tvdb_id`
- `idx_movies_year` on `tmdb_year`

**Unique Constraints:**
- `(tmdb_title, tmdb_year)` - Prevents duplicate movies

**Foreign Keys:**
- `processed_lines.movie_id` → `movies.id` (CASCADE)

---

### tvshows

Stores TV show metadata from TMDB with season/episode information.

| Column | Type | Constraints | Description |
|--------|------|-------------|-------------|
| `id` | INTEGER | PRIMARY KEY | Unique identifier |
| `tmdb_id` | INTEGER | NOT NULL | TMDB database ID |
| `tvdb_id` | INTEGER | NULLABLE | TVDB database ID (from TMDB external_ids) |
| `tmdb_title` | VARCHAR(255) | NOT NULL | Show title from TMDB |
| `tmdb_year` | INTEGER | NOT NULL | First air date year |
| `tmdb_genres` | TEXT | NULLABLE | Genres as JSON array |
| `season` | INTEGER | NULLABLE | Season number |
| `episode` | INTEGER | NULLABLE | Episode number |
| `created_at` | TIMESTAMP | NOT NULL | Record creation time |
| `updated_at` | TIMESTAMP | NOT NULL | Record update time |

**Indexes:**
- `idx_tvshows_tmdb` on `tmdb_id`
- `idx_tvshows_tvdb` on `tvdb_id`
- `idx_tvshows_season_episode` on `(season, episode)`

**Unique Constraints:**
- `(tmdb_title, tmdb_year, season, episode)` - Prevents duplicate episodes

**Foreign Keys:**
- `processed_lines.tvshow_id` → `tvshows.id` (CASCADE)

---

## Entity Relationships

```
processed_lines
    ├── movie_id → movies.id
    ├── tvshow_id → tvshows.id
    └── overrides_id → processed_lines.id (self-reference)

movies
    └── processed_lines[] (one-to-many)

tvshows
    └── processed_lines[] (one-to-many)
```

## Design Principles

### Polymorphic Relationships

The `processed_lines` table uses nullable foreign keys to establish relationships with different content types. Only one foreign key should be populated per record based on `content_type`.

### Data Separation

- **M3U-specific data**: Stored in `processed_lines` (original line content, URLs, parsing metadata)
- **Normalized metadata**: Stored in content-specific tables (`movies`, `tvshows`) from TMDB
- API responses include both via `raw` attribute containing the `processed_lines` data

### Deduplication Strategy

1. **M3U lines**: SHA-256 hash (`line_hash`) prevents duplicate playlist entries
2. **Movies**: Unique constraint on `(title, year)` prevents duplicate TMDB entries
3. **TV Shows**: Unique constraint on `(title, year, season, episode)` prevents duplicate episodes

### Version History

The `overrides_id` and `overrides_at` fields in `processed_lines` enable tracking when a line is superseded by a newer version, maintaining audit trail.

## Migration Strategy

GORM AutoMigrate handles schema creation and updates. For production:

1. Use migration tools (e.g., golang-migrate)
2. Version control all schema changes
3. Test migrations in staging environment
4. Backup data before migration

## Query Optimization

### Recommended Queries

**Find all movies from a specific group:**
```sql
SELECT m.* FROM movies m
JOIN processed_lines pl ON pl.movie_id = m.id
WHERE pl.group_title = 'VOD - Movies'
AND pl.state = 'processed';
```

**Find TV show episodes by season:**
```sql
SELECT t.* FROM tvshows t
JOIN processed_lines pl ON pl.tvshow_id = t.id
WHERE t.tmdb_title = 'Breaking Bad'
AND t.season = 1
ORDER BY t.episode;
```

**Check for duplicate lines:**
```sql
SELECT line_hash, COUNT(*) FROM processed_lines
GROUP BY line_hash
HAVING COUNT(*) > 1;
```

## Best Practices

1. **Always use transactions** for operations affecting multiple tables
2. **Populate content_type** before setting foreign keys
3. **Update line_hash** when line_content changes
4. **Check for existing TMDB entries** before creating new movies/tvshows
5. **Use prepared statements** to prevent SQL injection
6. **Index foreign keys** for optimal join performance

## Future Enhancements

Potential schema additions:

- `channels` table for live TV content
- `uncategorized` table for unclassified content
- `download_info` table for tracking downloads
- `user_preferences` table for filtering rules
- Full-text search indexes on titles
- Materialized views for statistics
