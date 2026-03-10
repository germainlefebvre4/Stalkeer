## 1. Fix Sonarr download path routing

- [x] 1.1 In `cmd/sonarr.go`, replace the `baseDestPath` construction: remove `cfg.Downloads.TVShowsPath` and `filepath.Base(series.Path)`, use `series.Path` directly as the root — `filepath.Join(series.Path, fmt.Sprintf("Season %02d", ...), fmt.Sprintf("... - S%02dE%02d", ...))`
- [x] 1.2 Add a guard: if `series.Path == ""`, fall back to the previous construction using `cfg.Downloads.TVShowsPath + filepath.Base(series.Title)` with a warning log

## 2. Fix Radarr download path routing

- [x] 2.1 In `cmd/radarr.go`, replace the `baseDestPath` construction: remove `cfg.Downloads.MoviesPath` and `filepath.Base(movie.Path)`, use `movie.Path` directly — `filepath.Join(movie.Path, fmt.Sprintf("%s (%d)", sanitizeFilename(movie.Title), movie.Year))`
- [x] 2.2 Add a guard: if `movie.Path == ""`, fall back to the previous construction using `cfg.Downloads.MoviesPath + title` with a warning log

## 3. Tests

- [x] 3.1 Update / add unit test for the Sonarr command path logic: assert that when `series.Path` is `/downloads/sonarr-bis/Malcolm in the Middle`, the destination begins with `/downloads/sonarr-bis/Malcolm in the Middle/`
- [x] 3.2 Update / add unit test for the Radarr command path logic: assert that when `movie.Path` is `/downloads/radarr-4k/Inception (2010)`, the destination begins with `/downloads/radarr-4k/Inception (2010)/`
- [x] 3.3 Add unit test for empty path fallback (both Sonarr and Radarr)

## 4. Documentation

- [x] 4.1 Update `config.yml.example` comments on `downloads.tvshows_path` and `downloads.movies_path` to clarify they are fallbacks; Sonarr/Radarr `series.Path` / `movie.Path` takes precedence
