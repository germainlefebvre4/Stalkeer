## MODIFIED Requirements

### Requirement: Strip quality suffixes from movie titles
Before querying TMDB, the processor SHALL remove quality and language suffixes from M3U titles. The following suffixes SHALL be stripped (case-insensitive, matched as whole words using word-boundary rules): `SD`, `HD`, `FHD`, `UHD`, `4K`, `MULTI`, `VOSTFR`, `VF`.

#### Scenario: Strip trailing SD suffix
- **WHEN** a movie title is `"Wonder Woman SD"`
- **THEN** the title sent to TMDB SHALL be `"Wonder Woman"`

#### Scenario: Strip trailing FHD MULTI suffix
- **WHEN** a movie title is `"Die Hart 2 (2024) FHD MULTI"`
- **THEN** the title sent to TMDB SHALL be `"Die Hart 2"` with year `2024`

#### Scenario: Strip trailing HD suffix with accented characters
- **WHEN** a movie title is `"Jumanji : Bienvenue dans la jungle SD"`
- **THEN** the title sent to TMDB SHALL be `"Jumanji : Bienvenue dans la jungle"`

#### Scenario: Title without suffix is unchanged
- **WHEN** a movie title has no quality suffix (e.g., `"Inception (2010)"`)
- **THEN** the title sent to TMDB SHALL be `"Inception"` with year `2010`

### Requirement: Resolution extraction uses word-boundary matching
The classifier's `ExtractResolution()` function SHALL use precompiled word-boundary regex patterns to detect resolution. Detection SHALL NOT use substring matching (`strings.Contains`) to avoid false positives on partial matches.

Recognized resolution values and their patterns (case-insensitive, whole-word):
- `"4K"`: matches `4K`, `UHD`, `2160p`
- `"1080p"`: matches `1080p`, `FullHD`, `FHD`
- `"720p"`: matches `720p`, `HD`
- `"480p"`: matches `480p`, `SD`

#### Scenario: HD matches 720p exactly
- **WHEN** a title is `"Inception HD"`
- **THEN** `ExtractResolution()` SHALL return `"720p"`

#### Scenario: FHD does not match 720p via partial HD match
- **WHEN** a title is `"Inception FHD"`
- **THEN** `ExtractResolution()` SHALL return `"1080p"` (not `"720p"`)

#### Scenario: UHD does not match 720p via partial HD match
- **WHEN** a title is `"Inception UHD"`
- **THEN** `ExtractResolution()` SHALL return `"4K"` (not `"720p"`)

#### Scenario: No quality marker returns nil
- **WHEN** a title is `"Inception"`
- **THEN** `ExtractResolution()` SHALL return `nil`
