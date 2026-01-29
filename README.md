# Stalkeer

> Parse M3U playlists and download missing media items from Radarr and Sonarr via direct links.

[![CI](https://github.com/glefebvre/stalkeer/workflows/Go%20CI/badge.svg)](https://github.com/glefebvre/stalkeer/actions)
[![Go Report Card](https://goreportcard.com/badge/github.com/glefebvre/stalkeer)](https://goreportcard.com/report/github.com/glefebvre/stalkeer)
[![License](https://img.shields.io/github/license/glefebvre/stalkeer)](LICENSE)

## Features

- 📺 Parse M3U playlist files for movies and TV shows
- 🗄️ Store media information in PostgreSQL database
- 🔍 Filter playlist items based on configurable patterns
- 🎬 Identify missing items from Radarr and Sonarr
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
# Using the binary
./bin/stalkeer

# Or using go run
go run cmd/main.go

# Check version
./bin/stalkeer version
```

### Using Docker Compose

```bash
# Start PostgreSQL
docker-compose up -d postgres

# Run the application
./bin/stalkeer
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
