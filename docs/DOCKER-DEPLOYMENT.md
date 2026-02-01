# Docker Deployment Guide

This guide explains how to deploy Stalkeer using Docker and Docker Compose.

## Quick Start

### 1. Prerequisites

- Docker Engine 20.10+
- Docker Compose 2.0+
- 2GB RAM minimum
- 10GB disk space for data

### 2. Basic Deployment

```bash
# Clone the repository
git clone https://github.com/yourusername/stalkeer.git
cd stalkeer

# Copy environment file
cp .env.example .env

# Edit configuration as needed
nano .env

# Start services
docker-compose up -d

# Check logs
docker-compose logs -f stalkeer

# Verify health
curl http://localhost:8080/health
```

## Environment Configuration

### Development Environment

Use `.env.development` for development:

```bash
cp .env.development .env
docker-compose up -d
```

### Production Environment

Use `.env.production` for production:

```bash
cp .env.production .env

# IMPORTANT: Update these values in .env
# - POSTGRES_PASSWORD: Use a strong password
# - DB_PASSWORD: Use the same strong password
# - TMDB_API_KEY: Your TMDB API key
# - RADARR_API_KEY: Your Radarr API key
# - SONARR_API_KEY: Your Sonarr API key

docker-compose up -d
```

## Configuration Options

### Database Configuration

```bash
# PostgreSQL settings
POSTGRES_DB=stalkeer          # Database name
POSTGRES_USER=postgres         # Database user
POSTGRES_PASSWORD=postgres     # Database password (CHANGE IN PRODUCTION!)
POSTGRES_PORT=5432            # Host port mapping

# Application database connection
DB_HOST=postgres              # Database hostname (container name)
DB_PORT=5432                  # Database port
DB_USER=postgres              # Database user
DB_PASSWORD=postgres          # Database password
DB_NAME=stalkeer             # Database name
DB_SSLMODE=disable           # SSL mode (use 'require' in production)
```

### Application Settings

```bash
API_PORT=8080                # API server port
LOG_LEVEL=info              # Log level: debug, info, warn, error
TMDB_API_KEY=               # TMDB API key for metadata enrichment
```

### External Services

```bash
# Radarr
RADARR_URL=http://radarr:7878
RADARR_API_KEY=your_api_key
RADARR_PORT=7878

# Sonarr
SONARR_URL=http://sonarr:8989
SONARR_API_KEY=your_api_key
SONARR_PORT=8989
```

### Volume Paths

```bash
DOWNLOADS_DIR=./data/downloads    # Downloads directory
TEMP_DIR=./data/temp             # Temporary files directory
```

## Architecture

### Services

- **postgres**: PostgreSQL 18 database
- **stalkeer**: Main application (API server)
- **radarr**: Movie management (optional, for development)
- **sonarr**: TV show management (optional, for development)

### Networks

All services run on the `stalkeer-network` bridge network for inter-container communication.

### Volumes

- `postgres_data`: Persistent PostgreSQL data
- `./config.yml`: Application configuration (mounted read-only)
- `./m3u_playlist`: M3U playlist files (mounted read-only)
- `./data/downloads`: Downloaded media files
- `./data/temp`: Temporary download files

## Health Checks

### Container Health

All services have health checks configured:

```bash
# Check service health
docker-compose ps

# Healthy output shows "healthy" status
```

### Application Health

```bash
# Health check endpoint
curl http://localhost:8080/health

# Expected response:
# {"status":"healthy"}
```

### Database Health

```bash
# PostgreSQL health check
docker-compose exec postgres pg_isready -U postgres

# Expected output:
# postgres:5432 - accepting connections
```

## Resource Limits

Default resource limits (can be adjusted in docker-compose.yml):

```yaml
limits:
  cpus: '2.0'
  memory: 2G
reservations:
  cpus: '0.5'
  memory: 512M
```

## Building Custom Images

### Build Locally

```bash
# Build with version
make docker-build-versioned VERSION=1.0.0

# Or use docker-compose
docker-compose build
```

### Build Arguments

- `VERSION`: Application version (default: dev)
- `COMMIT`: Git commit hash (default: unknown)
- `DATE`: Build date (default: unknown)

Example:

```bash
docker build \
  --build-arg VERSION=1.0.0 \
  --build-arg COMMIT=$(git rev-parse --short HEAD) \
  --build-arg DATE=$(date -u '+%Y-%m-%d_%H:%M:%S') \
  -t stalkeer:1.0.0 .
```

## Data Persistence

### Database Backups

```bash
# Backup database
docker-compose exec postgres pg_dump -U postgres stalkeer > backup.sql

# Restore database
docker-compose exec -T postgres psql -U postgres stalkeer < backup.sql
```

### Volume Backups

```bash
# Backup downloads
tar czf downloads-backup.tar.gz ./data/downloads

# Backup configuration
tar czf config-backup.tar.gz ./config.yml ./m3u_playlist
```

## Troubleshooting

### Container Won't Start

```bash
# Check logs
docker-compose logs stalkeer

# Common issues:
# 1. Database not ready: Wait for postgres health check
# 2. Configuration error: Check config.yml syntax
# 3. Port conflict: Change API_PORT in .env
```

### Database Connection Errors

```bash
# Check database status
docker-compose ps postgres

# Restart database
docker-compose restart postgres

# Check database logs
docker-compose logs postgres
```

### Permission Issues

```bash
# Fix ownership of data directories
sudo chown -R 1000:1000 ./data

# Verify permissions
ls -la ./data
```

## Security Considerations

### Production Checklist

- [ ] Change default database password
- [ ] Enable SSL for database (`DB_SSLMODE=require`)
- [ ] Use non-root database user
- [ ] Secure API keys in environment variables
- [ ] Enable firewall rules
- [ ] Use reverse proxy (nginx/traefik) for HTTPS
- [ ] Regular security updates (`docker-compose pull`)
- [ ] Monitor logs for suspicious activity
- [ ] Implement backup strategy
- [ ] Use Docker secrets for sensitive data (advanced)

### Secrets Management

For production, consider using Docker secrets:

```yaml
# docker-compose.yml (advanced)
secrets:
  db_password:
    file: ./secrets/db_password.txt
  tmdb_api_key:
    file: ./secrets/tmdb_api_key.txt

services:
  stalkeer:
    secrets:
      - db_password
      - tmdb_api_key
```

## Monitoring

### Container Metrics

```bash
# View resource usage
docker stats

# View specific container
docker stats stalkeer-app
```

### Application Logs

```bash
# Follow logs
docker-compose logs -f stalkeer

# Last 100 lines
docker-compose logs --tail=100 stalkeer

# JSON format logs (structured)
docker-compose logs stalkeer | jq
```

## Scaling

### Horizontal Scaling

For high availability, use Docker Swarm or Kubernetes:

```bash
# Docker Swarm example
docker stack deploy -c docker-compose.yml stalkeer
```

### Performance Tuning

Adjust in docker-compose.yml:

- `resources.limits.cpus`: CPU limit
- `resources.limits.memory`: Memory limit
- Database connection pool settings
- Download `max_parallel` setting

## Updates and Maintenance

### Update Application

```bash
# Pull latest code
git pull

# Rebuild and restart
docker-compose build stalkeer
docker-compose up -d stalkeer

# Verify
docker-compose logs -f stalkeer
```

### Update Dependencies

```bash
# Pull latest images
docker-compose pull

# Restart services
docker-compose up -d
```

## Advanced Topics

### Custom Network Configuration

```yaml
# docker-compose.yml
networks:
  stalkeer-network:
    driver: bridge
    ipam:
      config:
        - subnet: 172.20.0.0/16
```

### External Database

To use an external PostgreSQL database:

```bash
# .env
DB_HOST=external-db.example.com
DB_PORT=5432
DB_SSLMODE=require
```

Then comment out the `postgres` service in docker-compose.yml.

### Reverse Proxy Integration

Example nginx configuration:

```nginx
server {
    listen 80;
    server_name stalkeer.example.com;

    location / {
        proxy_pass http://localhost:8080;
        proxy_set_header Host $host;
        proxy_set_header X-Real-IP $remote_addr;
    }
}
```

## Support

For issues and questions:
- GitHub Issues: https://github.com/yourusername/stalkeer/issues
- Documentation: https://github.com/yourusername/stalkeer/docs
