## 1. Config

- [ ] 1.1 Add `RequestsPerSecond float64 \`mapstructure:"requests_per_second"\`` field to `TMDBConfig` in `internal/config/config.go`
- [ ] 1.2 Add `viper.SetDefault("tmdb.requests_per_second", 4.0)` in `setDefaults()`
- [ ] 1.3 Add `viper.BindEnv("tmdb.requests_per_second")` in `Load()`
- [ ] 1.4 Add `requests_per_second: 4.0` (with comment) to the `tmdb:` section of `config.yml`

## 2. TMDB Client — struct and constructor

- [ ] 2.1 Add `requestInterval time.Duration` field to `Client` struct in `internal/external/tmdb/tmdb.go`
- [ ] 2.2 Add `lastRequestAt time.Time` field to `Client` struct
- [ ] 2.3 Add `cache map[string][]byte` field to `Client` struct
- [ ] 2.4 Add `cacheMu sync.RWMutex` field to `Client` struct
- [ ] 2.5 Add `RequestsPerSecond float64` field to `Config` struct
- [ ] 2.6 In `NewClient`: compute `requestInterval` from `cfg.RequestsPerSecond` (skip if 0), initialise `cache` with `make(map[string][]byte)`

## 3. TMDB Client — makeRequest logic

- [ ] 3.1 At the top of `makeRequest`, add cache lookup: `cacheMu.RLock` → check `cache[requestURL]` → on hit, `RUnlock`, unmarshal cached bytes into `result`, return nil
- [ ] 3.2 After cache miss, add sleep-gap: if `requestInterval > 0`, compute `gap = requestInterval - time.Since(c.lastRequestAt)`; if `gap > 0`, `time.Sleep(gap)`; set `c.lastRequestAt = time.Now()`
- [ ] 3.3 Inside the `circuitBrk.Execute` closure, on 429: read `resp.Header.Get("Retry-After")`; try `strconv.Atoi` for seconds format; fall back to `http.ParseTime` for HTTP-date format; if `wait > 0`, `time.Sleep(wait)`
- [ ] 3.4 Capture the raw response body in an outer variable (`var rawBody []byte`) and assign it inside the closure after successful read
- [ ] 3.5 After `retry.Do` succeeds, write to cache: `cacheMu.Lock` → `c.cache[requestURL] = rawBody` → `cacheMu.Unlock`

## 4. Wire config through processor

- [ ] 4.1 In `internal/processor/processor.go`, add `RequestsPerSecond: cfg.TMDB.RequestsPerSecond` to the `tmdb.NewClient(tmdb.Config{...})` call

## 5. Tests

- [ ] 5.1 Add test for cache hit: second identical call should not make a new HTTP request
- [ ] 5.2 Add test for Retry-After seconds format: 429 response with `Retry-After: 1` should cause a sleep before retry
- [ ] 5.3 Add test for Retry-After HTTP-date format: 429 with date header parses correctly
- [ ] 5.4 Add test for `RequestsPerSecond: 0` disabling rate limiting (no sleep between calls)
