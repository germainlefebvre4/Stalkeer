## ADDED Requirements

### Requirement: Each CLI command is defined in its own file
The codebase SHALL organise each Cobra command into a dedicated file under `cmd/`. The file name SHALL reflect the command name (e.g., `server.go` for `serverCmd`, `radarr.go` for `radarrCmd`). Commands that share a cohesive domain MAY be grouped in one file (e.g., multiple M3U-related commands in `m3u.go`).

#### Scenario: New command added
- **WHEN** a developer adds a new CLI command
- **THEN** the command is defined in a new file under `cmd/` without modifying `cmd/main.go`

#### Scenario: Command file is self-contained
- **WHEN** reading any command file in `cmd/`
- **THEN** the file contains the command variable, all flag declarations, and the `rootCmd.AddCommand` call within a single `init()` function

### Requirement: Commands self-register via init()
Each command file SHALL call `rootCmd.AddCommand(<cmd>)` inside its own `init()` function. `cmd/main.go` SHALL NOT contain any `AddCommand` calls.

#### Scenario: Root command has no AddCommand calls
- **WHEN** reading `cmd/main.go`
- **THEN** no `rootCmd.AddCommand` call is present in that file

#### Scenario: Command registration is local
- **WHEN** reading a command file (e.g., `cmd/server.go`)
- **THEN** the file's `init()` both registers flags on the command and calls `rootCmd.AddCommand`

### Requirement: cmd/main.go is minimal
`cmd/main.go` SHALL contain only: `rootCmd` declaration, `initConfig()`, global persistent flags (e.g., `--config`), `cobra.OnInitialize`, and the `main()` function.

#### Scenario: main.go line count
- **WHEN** the refactor is complete
- **THEN** `cmd/main.go` contains fewer than 50 lines

### Requirement: Shared CLI helpers are isolated
Formatting helpers used across multiple command files (`formatBytes`, `sanitizeFilename`, `valueOrEmpty`) SHALL be defined in `cmd/format.go` and SHALL NOT be duplicated in command files.

#### Scenario: formatBytes is callable from any command file
- **WHEN** a command file needs to format a byte count
- **THEN** it calls `formatBytes()` which is defined in `cmd/format.go` within the same `package main`

### Requirement: No stdlib package name shadowing in internal/
`internal/` packages SHALL NOT use names that shadow Go standard library packages. Specifically:
- The package currently named `errors` SHALL be renamed to `apperrors`
- The package currently named `testing` SHALL be renamed to `testutil`

#### Scenario: Importing apperrors
- **WHEN** a package imports the application error utilities
- **THEN** the import path is `github.com/glefebvre/stalkeer/internal/apperrors` and no alias is required to avoid stdlib collision

#### Scenario: Importing testutil
- **WHEN** a test file imports the shared test helpers
- **THEN** the import path is `github.com/glefebvre/stalkeer/internal/testutil`
