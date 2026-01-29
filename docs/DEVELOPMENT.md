# Development Setup Guide

This guide will help you set up a development environment for Stalkeer.

## Prerequisites

- **Go**: Version 1.21 or higher
  - Installation: https://go.dev/doc/install
  - Verify: `go version`

- **PostgreSQL**: Version 12 or higher
  - Installation: https://www.postgresql.org/download/
  - Or use Docker: `docker-compose up -d postgres`

- **Git**: Version control
  - Installation: https://git-scm.com/downloads

- **Optional Tools**:
  - `golangci-lint` for linting: https://golangci-lint.run/usage/install/
  - `air` for live reloading: `go install github.com/cosmtrek/air@latest`

## Clone Repository

```bash
git clone https://github.com/glefebvre/stalkeer.git
cd stalkeer
```

## Install Dependencies

```bash
go mod download
```

## Database Setup

### Option 1: Using Docker Compose

The project includes a `docker-compose.yml` file:

```bash
docker-compose up -d postgres
```

This will start PostgreSQL on `localhost:5432` with:
- Username: `stalkeer`
- Password: `stalkeer`
- Database: `stalkeer`

### Option 2: Manual PostgreSQL Setup

1. Create a database:
```sql
CREATE DATABASE stalkeer;
CREATE USER stalkeer WITH PASSWORD 'your_password';
GRANT ALL PRIVILEGES ON DATABASE stalkeer TO stalkeer;
```

2. Update your configuration (see Configuration section below)

## Configuration

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
  sslmode: disable

m3u:
  file_path: /path/to/your/playlist.m3u
  update_interval: 3600

logging:
  level: debug
  format: json

api:
  port: 8080
```

Alternatively, use environment variables:
```bash
export STALKEER_DATABASE_USER=stalkeer
export STALKEER_DATABASE_PASSWORD=your_password
export STALKEER_DATABASE_DBNAME=stalkeer
export STALKEER_M3U_FILE_PATH=/path/to/playlist.m3u
```

## Build the Application

```bash
go build -o bin/stalkeer cmd/main.go
```

## Run the Application

```bash
./bin/stalkeer
```

Or using `go run`:
```bash
go run cmd/main.go
```

## Development Workflow

### Running Tests

```bash
# Run all tests
go test ./...

# Run tests with coverage
go test -cover ./...

# Run tests with race detection
go test -race ./...
```

See [TESTING.md](TESTING.md) for detailed testing documentation.

### Code Formatting

```bash
# Format all code
gofmt -w .

# Format and organize imports
goimports -w .
```

### Linting

```bash
# Run golangci-lint
golangci-lint run
```

### Live Reloading (Optional)

Install Air for live reloading during development:

```bash
go install github.com/cosmtrek/air@latest
```

Create `.air.toml`:
```toml
root = "."
tmp_dir = "tmp"

[build]
  cmd = "go build -o ./bin/stalkeer ./cmd/main.go"
  bin = "bin/stalkeer"
  include_ext = ["go", "yml"]
  exclude_dir = ["bin", "tmp", "vendor"]
  delay = 1000
```

Run with Air:
```bash
air
```

## Project Structure

```
stalkeer/
├── cmd/                    # Application entry points
│   └── main.go
├── internal/               # Private application code
│   ├── api/               # REST API handlers
│   ├── config/            # Configuration management
│   ├── database/          # Database connection and setup
│   ├── models/            # Data models
│   ├── parser/            # M3U parser
│   └── testing/           # Test helpers and fixtures
├── docs/                  # Documentation
├── bin/                   # Compiled binaries (gitignored)
├── config.yml.example     # Example configuration
├── docker-compose.yml     # Docker services
├── go.mod                 # Go module definition
└── README.md              # Project README
```

## Common Tasks

### Add a New Dependency

```bash
go get github.com/some/package
go mod tidy
```

### Update Dependencies

```bash
go get -u ./...
go mod tidy
```

### Generate Mock Data

Create a sample M3U file for testing:

```bash
cat > test.m3u << EOF
#EXTM3U
#EXTINF:-1 tvg-name="Test Movie" group-title="VOD - Movies" tvg-logo="http://example.com/logo.png",Test Movie
http://example.com/stream/movie1.mp4
#EXTINF:-1 tvg-name="Test Show S01E01" group-title="TV Shows",Test Show S01E01
http://example.com/stream/show1.mp4
EOF
```

## Troubleshooting

### Build Errors

**Issue**: `package not found`
```bash
go mod download
go mod tidy
```

**Issue**: `cannot find module`
```bash
go clean -modcache
go mod download
```

### Database Connection Issues

**Issue**: `connection refused`
- Verify PostgreSQL is running: `docker-compose ps` or `pg_isready -h localhost`
- Check connection details in `config.yml`

**Issue**: `password authentication failed`
- Verify credentials in configuration
- Check PostgreSQL user permissions

### Runtime Errors

**Issue**: `config.yml not found`
- Ensure `config.yml` exists in the current directory
- Or set environment variables

**Issue**: `m3u file not found`
- Verify the path in `config.yml`
- Ensure the file exists and is readable

## Next Steps

- Read the [Testing Guide](TESTING.md)
- Review the [API Documentation](API.md) (coming soon)
- Check the [Contributing Guidelines](CONTRIBUTING.md) (coming soon)

## Getting Help

- Open an issue on GitHub
- Check existing documentation
- Review the code comments and tests

## Additional Resources

- [Effective Go](https://go.dev/doc/effective_go)
- [Go Code Review Comments](https://go.dev/wiki/CodeReviewComments)
- [GORM Documentation](https://gorm.io/docs/)
- [Gin Documentation](https://gin-gonic.com/docs/)
