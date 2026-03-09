## 1. Model & Migration

- [x] 1.1 Add `Resolution *string` field to `ProcessedLine` in `internal/models/playlist_item.go`
- [x] 1.2 Remove `OverridesID`, `OverridesAt`, and `Overrides` fields from `ProcessedLine`
- [x] 1.3 Write DB migration: `ADD COLUMN resolution VARCHAR(10)`, `DROP COLUMN overrides_id`, `DROP COLUMN overrides_at`

## 2. Classifier Fix

- [x] 2.1 Rewrite `ExtractResolution()` in `internal/classifier/classifier.go` to iterate over `c.resolutionPatterns` instead of using `strings.Contains`
- [x] 2.2 Update or add tests in `internal/classifier/classifier_test.go` covering: HD→720p, FHD→1080p, UHD→4K, no marker→nil, and the false-positive cases (FHD not matching as 720p, UHD not matching as 720p)

## 3. Processor — Persist Resolution

- [x] 3.1 In `setContentType()` in `internal/processor/processor.go`, assign `line.Resolution = classification.Resolution` after calling `p.classifier.Classify()`
- [x] 3.2 Add or update processor tests in `internal/processor/processor_test.go` to verify that a line with a quality marker in its title has the correct `Resolution` value after processing

## 4. Matcher — Quality-Ordered Candidates

- [x] 4.1 Add `FindMovieDownloadCandidates(db *gorm.DB, movieID uint) ([]models.ProcessedLine, error)` to `internal/matcher/matcher.go`, querying with `ORDER BY CASE resolution WHEN '720p' THEN 1 WHEN '1080p' THEN 2 WHEN '4K' THEN 3 WHEN '480p' THEN 4 ELSE 5 END ASC, created_at DESC` and filtering `state IN ('processed', 'failed')`
- [x] 4.2 Add `FindTVShowDownloadCandidates(db *gorm.DB, tvshowID uint) ([]models.ProcessedLine, error)` with the same ordering
- [x] 4.3 Add tests in `internal/matcher/matcher_test.go` covering: correct ordering by quality, recency tiebreak within same quality, NULL resolution sorted last

## 5. Download Commands — Fallback Loop

- [x] 5.1 In the `download radarr` command (`cmd/main.go`): after matching the movie, call `FindMovieDownloadCandidates()` and replace the single-attempt download with a loop that tries each candidate in order, marks failures with `state = "failed"`, and stops on success
- [x] 5.2 In the `download sonarr` command (`cmd/main.go`): same fallback loop using `FindTVShowDownloadCandidates()`
- [x] 5.3 Ensure the output log shows which candidate (quality + attempt number) is being tried, e.g. `→ attempt 1/3 (720p) : http://...`
