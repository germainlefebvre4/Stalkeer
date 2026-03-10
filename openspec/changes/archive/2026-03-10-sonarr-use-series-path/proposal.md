## Why

When Sonarr manages series across multiple root folders (e.g. `/downloads/sonarr` and `/downloads/sonarr-bis`), Stalkeer ignores the root folder information and always writes downloads to the single `downloads.tvshows_path` configured in `config.yml`. As a result, series assigned to a secondary root folder land in the wrong directory.

## What Changes

- The Sonarr download command will derive the destination path directly from `series.Path` (the full absolute path returned by the Sonarr API) instead of combining `cfg.Downloads.TVShowsPath` with `filepath.Base(series.Path)`.
- `downloads.tvshows_path` in `config.yml` is no longer used for Sonarr-sourced downloads; the path authority shifts entirely to Sonarr.
- The same fix is applied consistently for Radarr: `movie.Path` (if available) is used directly in place of `cfg.Downloads.MoviesPath + title`.

## Capabilities

### New Capabilities

- `sonarr-series-path-routing`: Use `series.Path` from the Sonarr API as the authoritative destination directory for TV show downloads, supporting multiple root folders without additional configuration.

### Modified Capabilities

<!-- No existing spec-level behavior changes. -->

## Impact

- `cmd/sonarr.go`: `baseDestPath` construction changed — remove `cfg.Downloads.TVShowsPath` + `filepath.Base(series.Path)`, replace with `series.Path` directly.
- `cmd/radarr.go`: same pattern audit for `movie.Path`.
- `config.yml` / `DownloadsConfig`: `tvshows_path` becomes optional / superseded by Sonarr's own path management.
- No database schema changes required.
- No API changes.
