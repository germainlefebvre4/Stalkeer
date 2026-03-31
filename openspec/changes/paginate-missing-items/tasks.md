## 1. Sonarr client — pagination

- [ ] 1.1 Add `Logger *logger.Logger` to `sonarr.Config` and `logger *logger.Logger` to `sonarr.Client`; populate in `New()`
- [ ] 1.2 Update `getEpisodes` to decode `totalRecords` from the paged response and return `([]Episode, int, error)`
- [ ] 1.3 Replace the single-request `GetMissingEpisodes` body with a page loop: build endpoint per page, call `retry.Do` per page, append records, break when `len(all) >= totalRecords` or empty page
- [ ] 1.4 Add INFO log inside the loop when `c.logger != nil`

## 2. Radarr client — pagination

- [ ] 2.1 Add `Logger *logger.Logger` to `radarr.Config` and `logger *logger.Logger` to `radarr.Client`; populate in `New()`
- [ ] 2.2 Add private helper `getPagedMovies(ctx, endpoint) ([]Movie, int, error)` that decodes the paged `wanted/missing` envelope
- [ ] 2.3 Replace `GetMissingMovies` to use `/api/v3/wanted/missing` endpoint and the same page loop pattern as Sonarr; remove client-side `monitored && !hasFile` filter
- [ ] 2.4 Add INFO log inside the loop when `c.logger != nil`

## 3. CLI wiring

- [ ] 3.1 Pass logger to `sonarr.Config{Logger: log}` in `cmd/sonarr.go` (if a logger instance is available; verify how other commands obtain it)
- [ ] 3.2 Pass logger to `radarr.Config{Logger: log}` in `cmd/radarr.go`

## 4. Tests

- [ ] 4.1 Update Sonarr unit test server to return paged response `{"totalRecords": N, "records": [...]}` for `wanted/missing`
- [ ] 4.2 Add Sonarr test: multi-page scenario (e.g. totalRecords=3, pageSize=2 → 2 requests)
- [ ] 4.3 Update Radarr unit test server to serve `wanted/missing` paged responses
- [ ] 4.4 Add Radarr test: multi-page scenario
- [ ] 4.5 Add test: empty `totalRecords=0` returns empty slice for both clients

## 5. Build & verification

- [ ] 5.1 Run `go build -o bin/stalkeer cmd/main.go` and confirm it compiles
- [ ] 5.2 Run full test suite (`go test ./...`) and confirm all tests pass
