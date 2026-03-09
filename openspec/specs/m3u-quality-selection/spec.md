# Capability: M3U Quality Selection

## Purpose

Defines how the system selects among multiple M3U entries for the same content (same movie or TV episode) based on resolution preference, and how it handles download failures by falling back to lower-priority candidates.

## Requirements

### Requirement: Resolution is persisted on ProcessedLine
The processor SHALL store the detected resolution of each M3U entry in the `resolution` field of `ProcessedLine`. Valid values are `"4K"`, `"1080p"`, `"720p"`, `"480p"`, or `NULL` when no resolution can be detected.

#### Scenario: Resolution stored for a 720p entry
- **WHEN** an M3U entry title contains a `720p` or `HD` quality marker
- **THEN** the resulting `ProcessedLine` SHALL have `resolution = "720p"`

#### Scenario: Resolution stored for a 4K entry
- **WHEN** an M3U entry title contains `4K`, `UHD`, or `2160p`
- **THEN** the resulting `ProcessedLine` SHALL have `resolution = "4K"`

#### Scenario: No resolution marker yields NULL
- **WHEN** an M3U entry title contains no recognized quality marker
- **THEN** the resulting `ProcessedLine` SHALL have `resolution = NULL`

### Requirement: Quality-ordered candidate list for download
When selecting a URL to download a given movie or TV episode, the system SHALL return all eligible `ProcessedLine` records for that content ordered by quality preference, then by recency within the same quality tier.

Quality preference order (ascending priority): `720p` (1) → `1080p` (2) → `4K` (3) → `480p` (4) → `NULL` (5).

Eligible candidates are `ProcessedLine` records with `state IN ('processed', 'failed')`.

#### Scenario: 720p candidate is selected first when available
- **WHEN** a movie has ProcessedLine entries at 720p, 1080p, and 4K
- **THEN** the 720p entry SHALL be returned as the first candidate

#### Scenario: Most recent entry preferred within same quality tier
- **WHEN** a movie has two ProcessedLine entries both at 720p, added at different times
- **THEN** the one with the later `created_at` SHALL be returned first

#### Scenario: Falls back to 1080p when no 720p exists
- **WHEN** a movie has ProcessedLine entries at 1080p and 4K, but none at 720p
- **THEN** the 1080p entry SHALL be returned as the first candidate

#### Scenario: NULL resolution candidates sorted last
- **WHEN** a movie has a ProcessedLine at 1080p and another with NULL resolution
- **THEN** the 1080p entry SHALL precede the NULL-resolution entry

### Requirement: Download fallback loop over quality candidates
The download commands (`download radarr`, `download sonarr`) SHALL attempt each candidate URL in quality-preference order. On download failure, the failed `ProcessedLine` SHALL be marked `state = "failed"` and the next candidate SHALL be attempted. The loop stops on the first successful download.

#### Scenario: First candidate fails, second succeeds
- **WHEN** the preferred 720p URL returns a network error
- **THEN** its `ProcessedLine.state` SHALL be set to `"failed"` and the next candidate (1080p) SHALL be attempted

#### Scenario: All candidates fail
- **WHEN** every candidate URL fails to download
- **THEN** the content item SHALL be counted as failed and the command SHALL continue to the next content item without crashing

#### Scenario: Successful download stops the loop
- **WHEN** the 720p candidate downloads successfully
- **THEN** no further candidates SHALL be attempted for that content item
