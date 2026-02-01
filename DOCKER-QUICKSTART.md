# Quick Start with Docker

This guide will help you get Stalkeer up and running with Docker in minutes.

## Prerequisites

- Docker Engine 20.10+
- Docker Compose 2.0+

## 5-Minute Setup

### 1. Clone and Configure

```bash
# Clone the repository
git clone https://github.com/yourusername/stalkeer.git
cd stalkeer

# Copy and edit environment file
cp .env.example .env
```

### 2. Configure M3U Playlist

Create or copy your M3U playlist file:

```bash
# Place your M3U file in the m3u_playlist directory
cp /path/to/your/playlist.m3u ./m3u_playlist/

# Or use the sample for testing
# Already included: ./m3u_playlist/sample.m3u
```

### 3. Start Services

```bash
# Start all services
docker-compose up -d

# View logs
docker-compose logs -f stalkeer
```

### 4. Verify Setup

```bash
# Check health
curl http://localhost:8080/health

# Expected response:
# {"status":"healthy"}
```

## Access Services

- **Stalkeer API**: http://localhost:8080
- **API Documentation**: http://localhost:8080/api/v1
- **Health Check**: http://localhost:8080/health

### Optional Development Services

If you uncommented optional services in docker-compose.yml:

- **Radarr**: http://localhost:7878
- **Sonarr**: http://localhost:8989
- **Prowlarr**: http://localhost:9696 (if enabled)

## Basic Operations

### Process M3U Playlist

```bash
# Using docker-compose
docker-compose exec stalkeer /app/stalkeer process /app/m3u_playlist/sample.m3u

# Or run directly
docker run --rm \
  -v $(pwd)/config.yml:/app/config/config.yml:ro \
  -v $(pwd)/m3u_playlist:/app/m3u_playlist:ro \
  stalkeer:test process /app/m3u_playlist/sample.m3u
```

### Query API

```bash
# List movies
curl http://localhost:8080/api/v1/movies

# List TV shows
curl http://localhost:8080/api/v1/tvshows

# Get statistics
curl http://localhost:8080/api/v1/stats
```

### View Logs

```bash
# Follow all logs
docker-compose logs -f

# Only Stalkeer logs
docker-compose logs -f stalkeer

# Last 100 lines
docker-compose logs --tail=100 stalkeer
```

## Configuration

### Minimal Required Configuration

Edit `.env` file:

```bash
# Database (use strong password in production)
POSTGRES_PASSWORD=your_strong_password
DB_PASSWORD=your_strong_password

# TMDB API (optional but recommended)
TMDB_API_KEY=your_tmdb_api_key
```

### Full Configuration Options

See [Docker Deployment Guide](docs/DOCKER-DEPLOYMENT.md) for complete configuration options.

## Common Tasks

### Stop Services

```bash
docker-compose down
```

### Restart Services

```bash
docker-compose restart
```

### Update to Latest

```bash
git pull
docker-compose build
docker-compose up -d
```

### View Database

```bash
# Access PostgreSQL
docker-compose exec postgres psql -U postgres stalkeer

# Run query
stalkeer=# SELECT COUNT(*) FROM movies;
```

### Backup Data

```bash
# Backup database
docker-compose exec postgres pg_dump -U postgres stalkeer > backup.sql

# Backup downloads
tar czf downloads-backup.tar.gz ./data/downloads
```

## Troubleshooting

### Services Won't Start

```bash
# Check status
docker-compose ps

# Check logs for errors
docker-compose logs
```

### Database Connection Issues

```bash
# Verify database is healthy
docker-compose ps postgres

# Check database logs
docker-compose logs postgres

# Restart database
docker-compose restart postgres
```

### Port Conflicts

If port 8080 is already in use:

```bash
# Edit .env
API_PORT=8090

# Restart
docker-compose up -d
```

### Reset Everything

```bash
# Stop and remove all containers and volumes
docker-compose down -v

# Start fresh
docker-compose up -d
```

## Production Deployment

For production deployment, see:
- [Docker Deployment Guide](docs/DOCKER-DEPLOYMENT.md) - Complete deployment guide
- [Security Best Practices](docs/DOCKER-DEPLOYMENT.md#security-considerations)
- [Performance Tuning](docs/DOCKER-DEPLOYMENT.md#performance-tuning)

## Next Steps

1. Configure Radarr/Sonarr integration
2. Set up scheduled processing
3. Configure filters
4. Set up monitoring
5. Configure backups

For detailed documentation, see:
- [Development Guide](docs/DEVELOPMENT.md)
- [API Documentation](docs/API.md)
- [Configuration Guide](docs/CONFIGURATION.md)
