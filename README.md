# Stalkeer

> Parse M3U playlists and download missing media items from Radarr and Sonarr via direct links.

[![CI](https://github.com/glefebvre/stalkeer/workflows/Go%20CI/badge.svg)](https://github.com/glefebvre/stalkeer/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/glefebvre/stalkeer)](https://goreportcard.com/report/github.com/glefebvre/stalkeer)
[![License](https://img.shields.io/github/license/glefebvre/stalkeer)](LICENSE)

## Features

- 📺 Parse M3U playlist files for movies and TV shows
- 🎬 **TMDB Integration** - Enrich media metadata with title, year, genres, posters, and more
- 🗄️ Store media information in PostgreSQL database
- 🔍 Filter playlist items based on configurable patterns
- 🌍 Multi-language support for TMDB metadata (English, French, Spanish, etc.)
- 🎯 Identify missing items from Radarr and Sonarr
- ⬇️ Download missing items via direct links
- 🚀 REST API for querying and managing media items
- 📊 Processing logs and statistics

## Quick Start

### Prerequisites

- Go 1.21 or higher
- PostgreSQL 12 or higher
- M3U playlist file

### Installation

```bash
# Clone the repository
git clone https://github.com/glefebvre/stalkeer.git
cd stalkeer

# Install dependencies
go mod download

# Build the application
make build
```

### Configuration

1. Copy the example configuration:
```bash
cp config.yml.example config.yml
```

2. Edit `config.yml` with your settings:
```yaml
database:
  host: localhost
  port: 5432
  user: stalkeer
  password: your_password
  dbname: stalkeer

m3u:
  file_path: /path/to/playlist.m3u
  update_interval: 3600

tmdb:
  enabled: true
  api_key: your_tmdb_api_key  # Get from https://www.themoviedb.org/settings/api
  language: en-US  # Language for metadata (en-US, fr-FR, es-ES, etc.)

api:
  port: 8080
```

Or use environment variables:
```bash
export STALKEER_DATABASE_USER=stalkeer
export STALKEER_DATABASE_PASSWORD=your_password
export STALKEER_DATABASE_DBNAME=stalkeer
export STALKEER_M3U_FILE_PATH=/path/to/playlist.m3u
```

### Running

```bash
# Process an M3U playlist file and store to database
./bin/stalkeer process /path/to/playlist.m3u

# Process with TMDB enrichment in French
./bin/stalkeer process /path/to/playlist.m3u --tmdb-language fr-FR

# Process without TMDB enrichment (faster)
./bin/stalkeer process /path/to/playlist.m3u --skip-tmdb

# Process with limit
./bin/stalkeer process /path/to/playlist.m3u --limit 100

# Force re-processing of existing entries
./bin/stalkeer process /path/to/playlist.m3u --force

# Dry-run analysis without database changes
./bin/stalkeer dryrun /path/to/playlist.m3u --limit 100

# Start the REST API server
./bin/stalkeer server --port 8080

# Run database migrations
./bin/stalkeer migrate

# Resume incomplete or failed downloads
./bin/stalkeer resume-downloads

# Resume downloads with options
./bin/stalkeer resume-downloads --limit 10 --parallel 5 --verbose

# Preview what would be resumed (dry-run)
./bin/stalkeer resume-downloads --dry-run

# Integrate with Radarr - resume incomplete downloads first
./bin/stalkeer radarr --resume --limit 20

# Integrate with Sonarr - resume incomplete downloads first
./bin/stalkeer sonarr --resume --limit 20

# Check version
./bin/stalkeer version

# Get help
./bin/stalkeer --help
./bin/stalkeer process --help
```

### CLI Commands

#### process

Process an M3U playlist file, classify content, enrich with TMDB metadata, and store entries in the database:

```bash
stalkeer process [m3u file] [flags]

Flags:
      --force              re-process existing entries
      --limit int          maximum number of items to process (0 = no limit)
      --batch-size int     batch size for database inserts (default 100)
      --progress int       show progress every N entries (default 1000)
      --skip-tmdb          skip TMDB metadata enrichment
      --tmdb-language      TMDB API language (e.g., 'en-US', 'fr-FR')
```

Example output:
```
=== Processing Complete ===
Total lines in file:  1000
Successfully processed: 985
Duplicates skipped:   5
Filtered out:         8
Errors:               2

Content breakdown:
  Movies:        742
  TV Shows:      231
  Channels:      0
  Uncategorized: 12

TMDB Enrichment:
  Matched:       891
  Not found:     76
  Errors:        6
  Match rate:    92.1%

Processing time: 1.2s
```

#### resume-downloads

Resume incomplete or failed downloads that were interrupted:

```bash
stalkeer resume-downloads [flags]

Flags:
      --dry-run             preview downloads without executing
      --limit int           maximum number of downloads to process (0 = no limit)
      --parallel int        number of concurrent downloads (default 3)
      --max-retries int     maximum retry attempts (default 5)
      --clean-stale-locks   clean up stale download locks before resuming (default true)
  -v, --verbose             verbose output
      --service string      filter by service type: all, radarr, sonarr (default "all")
```

The resume-downloads command identifies and resumes downloads that:
- Were interrupted by application shutdown or crashes
- Failed due to temporary network issues
- Are in pending, downloading, paused, or failed states
- Haven't exceeded the maximum retry limit

Example usage:
```bash
# Resume all incomplete downloads
stalkeer resume-downloads

# Preview what would be resumed
stalkeer resume-downloads --dry-run --verbose

# Resume up to 10 downloads with 5 concurrent workers
stalkeer resume-downloads --limit 10 --parallel 5

# Resume only failed Radarr downloads
stalkeer resume-downloads --service radarr
```

#### radarr

Download missing movies from Radarr by matching against M3U playlist:

```bash
stalkeer radarr [flags]

Flags:
      --dry-run      preview matches without downloading
      --limit int    maximum number of movies to process (0 = no limit)
      --parallel int number of concurrent downloads (default 3)
      --force        re-download existing files
  -v, --verbose      verbose output
      --resume       resume incomplete downloads before fetching new items
```

#### sonarr

Download missing TV show episodes from Sonarr by matching against M3U playlist:

```bash
stalkeer sonarr [flags]

Flags:
      --dry-run       preview matches without downloading
      --limit int     maximum number of episodes to process (0 = no limit)
      --parallel int  number of concurrent downloads (default 3)
      --force         re-download existing files
  -v, --verbose       verbose output
      --series-id int filter to specific Sonarr series ID
      --resume        resume incomplete downloads before fetching new episodes
```

#### dryrun

Analyze M3U playlist file without making database changes:

```bash
stalkeer dryrun [m3u file] [flags]

Flags:
      --limit int   maximum number of items to analyze (default 100)
```

#### server

Start the REST API server:

```bash
stalkeer server [flags]

Flags:
  -p, --port int       port to run the server on (default 8080)
  -a, --address string address to bind the server to (default "0.0.0.0")
```

#### migrate

Run database migrations:

```bash
stalkeer migrate
```

### Using Docker Compose

```bash
# Start PostgreSQL
docker-compose up -d postgres

# Run the application
./bin/stalkeer process
```

## Development

### Project Structure

```
stalkeer/
├── cmd/                    # Application entry points
│   └── main.go
├── internal/               # Private application code
│   ├── api/               # REST API handlers
│   ├── config/            # Configuration management
│   ├── database/          # Database connection and migrations
│   ├── models/            # Data models
│   ├── parser/            # M3U parser
│   └── testing/           # Test helpers and fixtures
├── docs/                  # Documentation
├── .github/               # GitHub workflows and templates
├── config.yml.example     # Example configuration
├── docker-compose.yml     # Docker services
├── Makefile              # Build automation
└── README.md             # This file
```

### Building

```bash
# Build the binary
make build

# Run tests
make test

# Generate coverage report
make coverage

# Format code
make fmt

# Run linters
make lint

# Clean build artifacts
make clean
```

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with race detection
go test -race ./...
```

See [docs/TESTING.md](docs/TESTING.md) for detailed testing documentation.

### Development Setup

See [docs/DEVELOPMENT.md](docs/DEVELOPMENT.md) for detailed development setup instructions.

## API Documentation

The REST API provides endpoints for managing processed M3U lines, movies, and TV shows.

### Health Check

```bash
GET /health
```

### Processed Lines

```bash
GET /api/v1/lines       # List all processed M3U lines
GET /api/v1/lines/:id   # Get line by ID
```

### Movies

```bash
GET  /api/v1/movies     # List all movies
POST /api/v1/movies     # Create a new movie
```

### TV Shows

```bash
GET /api/v1/tvshows     # List all TV shows
```

### Statistics

```bash
GET /api/v1/stats       # Get processing statistics
```

## Configuration

### Database Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `database.host` | string | `localhost` | PostgreSQL host |
| `database.port` | int | `5432` | PostgreSQL port |
| `database.user` | string | - | Database user (required) |
| `database.password` | string | - | Database password |
| `database.dbname` | string | - | Database name (required) |
| `database.sslmode` | string | `disable` | SSL mode |

### M3U Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `m3u.file_path` | string | - | Path to M3U playlist file (required) |
| `m3u.update_interval` | int | `3600` | Update interval in seconds |

### Logging Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `logging.level` | string | `info` | Log level (debug, info, warn, error) |
| `logging.format` | string | `json` | Log format (json, text) |

### API Configuration

| Field | Type | Default | Description |
|-------|------|---------|-------------|
| `api.port` | int | `8080` | API server port |

## Environment Variables

All configuration options can be overridden with environment variables using the `STALKEER_` prefix:

- `STALKEER_DATABASE_HOST`
- `STALKEER_DATABASE_PORT`
- `STALKEER_DATABASE_USER`
- `STALKEER_DATABASE_PASSWORD`
- `STALKEER_DATABASE_DBNAME`
- `STALKEER_M3U_FILE_PATH`
- `STALKEER_LOGGING_LEVEL`
- `STALKEER_API_PORT`

Or use a PostgreSQL connection string:
```bash
export DATABASE_URL="postgres://user:password@localhost:5432/stalkeer"
```

## Contributing

Contributions are welcome! Please read our contributing guidelines (coming soon) before submitting pull requests.

1. Fork the repository
2. Create your feature branch (`git checkout -b feature/amazing-feature`)
3. Commit your changes (`git commit -m 'Add some amazing feature'`)
4. Push to the branch (`git push origin feature/amazing-feature`)
5. Open a Pull Request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- [GORM](https://gorm.io/) - ORM library for Go
- [Gin](https://gin-gonic.com/) - HTTP web framework
- [Cobra](https://github.com/spf13/cobra) - CLI framework
- [Viper](https://github.com/spf13/viper) - Configuration management

## Support

- 📖 [Documentation](docs/)
- 🐛 [Issue Tracker](https://github.com/glefebvre/stalkeer/issues)
- 💬 [Discussions](https://github.com/glefebvre/stalkeer/discussions)
