## Context

Both `cmd/sonarr.go` and `cmd/radarr.go` construct the download destination path by combining a static config value (`cfg.Downloads.TVShowsPath` / `cfg.Downloads.MoviesPath`) with only the base name of the series/movie folder:

```go
// sonarr.go (current, broken)
baseDestPath := filepath.Join(
    cfg.Downloads.TVShowsPath,       // "./data/downloads/sonarr"  ← always this
    filepath.Base(series.Path),      // "Malcolm in the Middle"    ← discards root
    fmt.Sprintf("Season %02d", ...),
    fmt.Sprintf("...", ...),
)
```

Sonarr and Radarr expose `series.Path` / `movie.Path` as the **full absolute path** already configured in their Media Management settings (e.g. `/downloads/sonarr-bis/Malcolm in the Middle`). This path already encodes the correct root folder selection.

The paths Stalkeer writes to and the paths Sonarr/Radarr use share the same filesystem (same volume mounts), so Sonarr's path is directly usable.

## Goals / Non-Goals

**Goals:**
- Download TV show episodes to the directory Sonarr configured for that series, regardless of how many root folders exist.
- Download movies to the directory Radarr configured for that movie, regardless of how many root folders exist.
- No new configuration required — path authority is fully delegated to Sonarr/Radarr.

**Non-Goals:**
- Path remapping / translation (e.g. Docker volume aliasing) — not needed given shared filesystem.
- Removing `downloads.tvshows_path` / `downloads.movies_path` from config — kept as fallback.
- Radarr multi-root-folder support beyond what `movie.Path` already provides.

## Decisions

### Decision 1: Use `series.Path` / `movie.Path` directly as the base

**Chosen**: Replace `filepath.Join(cfg.Downloads.TVShowsPath, filepath.Base(series.Path), ...)` with `filepath.Join(series.Path, ...)`.

**Rationale**: `series.Path` is `{root_folder}/{series_name}` — it already contains the correctly chosen root folder. Using it directly requires zero additional configuration and is always in sync with what the user set in Sonarr's UI.

**Alternative considered**: Root-folder mapping table in `config.yml` (map Sonarr path prefix → local path prefix). Rejected — unnecessary complexity given identical filesystem layout, introduces config drift risk.

**Alternative considered**: Query `/api/v3/rootfolder` from Sonarr API and auto-discover mappings. Rejected — adds an extra API call on every run, still requires identical mount points, no net benefit over Option A.

### Decision 2: Apply the same fix to Radarr

`movie.Path` from the Radarr API has the same structure (`{root_folder}/{movie_folder}`). The fix is symmetrical: `filepath.Join(movie.Path, filepath.Base(movie.Path))` for the file name inside the folder, which collapses to just `movie.Path` as the destination (downloader appends extension).

### Decision 3: Keep `downloads.tvshows_path` / `downloads.movies_path` in config

These fields are retained for any future use (e.g. a hypothetical "download without Sonarr/Radarr" mode). They are simply no longer used in the primary download path construction in the Sonarr/Radarr commands.

## Risks / Trade-offs

| Risk | Mitigation |
|------|-----------|
| `series.Path` is empty string for some series | Guard with `if series.Path == ""` fallback to old behavior using `cfg.Downloads.TVShowsPath + filepath.Base(...)` |
| Different mount points between Sonarr and Stalkeer containers | Document assumption; user must ensure volume paths match |
| Radarr `movie.Path` contains the full movie directory (not just root), so `filepath.Base` used for filename only | The downloader already appends the extension; `movie.Path` alone is the correct base path |

## Migration Plan

1. No schema migration.
2. No API changes.
3. Existing downloads at the wrong path are not moved automatically — users can move them manually if desired.
4. On next run, new downloads land in the correct path; Sonarr's "Manual Import" or its scan will find them.
