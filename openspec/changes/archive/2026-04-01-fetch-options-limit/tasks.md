## 1. Sonarr — FetchOptions + early exit

- [x] 1.1 Add `FetchOptions` struct to `internal/external/sonarr/sonarr.go` with `Limit int` field
- [x] 1.2 Update `GetMissingEpisodes` signature to `GetMissingEpisodes(ctx context.Context, opts FetchOptions) ([]Episode, error)`
- [x] 1.3 Add early-exit condition in the pagination loop: if `opts.Limit > 0 && len(all) >= opts.Limit`, trim to `opts.Limit` and break
- [x] 1.4 Update `internal/external/sonarr/sonarr_test.go` — all `GetMissingEpisodes` call sites pass `FetchOptions{}`; add new test `TestGetMissingEpisodesWithLimit` covering early exit and trim scenarios

## 2. Radarr — FetchOptions + early exit

- [x] 2.1 Add `FetchOptions` struct to `internal/external/radarr/radarr.go` with `Limit int` field
- [x] 2.2 Update `GetMissingMovies` signature to `GetMissingMovies(ctx context.Context, opts FetchOptions) ([]Movie, error)`
- [x] 2.3 Add early-exit condition in the pagination loop: if `opts.Limit > 0 && len(all) >= opts.Limit`, trim to `opts.Limit` and break
- [x] 2.4 Update `internal/external/radarr/radarr_test.go` — all `GetMissingMovies` call sites pass `FetchOptions{}`; add new test `TestGetMissingMoviesWithLimit` covering early exit and trim scenarios

## 3. Command layer — wire up FetchOptions

- [x] 3.1 `cmd/sonarr.go`: replace `sonarrClient.GetMissingEpisodes(ctx)` with `sonarrClient.GetMissingEpisodes(ctx, sonarr.FetchOptions{Limit: limit})`; remove the post-fetch "Apply limit" block
- [x] 3.2 `cmd/radarr.go`: replace `radarrClient.GetMissingMovies(ctx)` with `radarrClient.GetMissingMovies(ctx, radarr.FetchOptions{Limit: limit})`; remove the post-fetch "Apply limit" block

## 4. Verification

- [x] 4.1 Run full test suite (`go test ./...`) and confirm all tests pass
