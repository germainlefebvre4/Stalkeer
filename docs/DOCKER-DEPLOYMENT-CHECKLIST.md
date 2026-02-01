# Docker Deployment Checklist

Use this checklist when deploying Stalkeer with Docker.

## Pre-Deployment

### Development Environment
- [ ] Docker Engine 20.10+ installed
- [ ] Docker Compose 2.0+ installed
- [ ] Clone repository: `git clone https://github.com/yourusername/stalkeer.git`
- [ ] Copy environment file: `cp .env.development .env`
- [ ] Place M3U playlist in `./m3u_playlist/`
- [ ] Review and adjust `config.yml` if needed

### Production Environment
- [ ] Docker Engine 20.10+ installed
- [ ] Docker Compose 2.0+ installed
- [ ] Clone repository to production server
- [ ] Copy environment file: `cp .env.production .env`
- [ ] **Change default passwords in `.env`**
- [ ] Set TMDB API key in `.env`
- [ ] Set Radarr/Sonarr API keys in `.env`
- [ ] Configure volume paths for data persistence
- [ ] Review security settings
- [ ] Configure reverse proxy (nginx/traefik) for HTTPS
- [ ] Set up firewall rules
- [ ] Configure backup strategy

## Build & Test

- [ ] Build Docker image: `docker build -t stalkeer:test .`
- [ ] Verify image size: `docker images | grep stalkeer`
  - Expected: ~85MB
- [ ] Test container start: `docker run --rm stalkeer:test version`
- [ ] Run configuration tests: `go test ./internal/config/...`

## Deployment

### Initial Deployment
- [ ] Start services: `docker-compose up -d`
- [ ] Check service status: `docker-compose ps`
  - All services should show "healthy" status
- [ ] View logs: `docker-compose logs -f`
- [ ] Test health endpoint: `curl http://localhost:8080/health`
  - Expected: `{"status":"healthy"}`
- [ ] Verify database connection: `docker-compose exec postgres pg_isready`
- [ ] Test API: `curl http://localhost:8080/api/v1/stats`

### Post-Deployment
- [ ] Process initial M3U playlist
- [ ] Verify data in database
- [ ] Test Radarr/Sonarr integration (if configured)
- [ ] Set up monitoring
- [ ] Configure log aggregation
- [ ] Set up automated backups
- [ ] Document deployment configuration

## Configuration Verification

### Environment Variables
- [ ] Database credentials configured
- [ ] API port configured (default: 8080)
- [ ] Log level appropriate for environment
- [ ] TMDB API key set (if using metadata enrichment)
- [ ] Radarr URL and API key set (if using)
- [ ] Sonarr URL and API key set (if using)
- [ ] Volume paths correct

### Docker Compose
- [ ] Service dependencies correct
- [ ] Health checks configured
- [ ] Volumes mounted correctly
- [ ] Network configuration correct
- [ ] Resource limits set appropriately
- [ ] Restart policy set (unless-stopped or always)

### Database
- [ ] PostgreSQL starts successfully
- [ ] Database initialized with schema
- [ ] Migrations run automatically
- [ ] Connection pool configured
- [ ] SSL mode appropriate for environment

## Security Checklist

### Production Security
- [ ] Changed default database password
- [ ] Using strong passwords (16+ characters)
- [ ] Database SSL enabled (`DB_SSLMODE=require`)
- [ ] API keys secured in environment variables
- [ ] Containers run as non-root user
- [ ] Configuration mounted read-only
- [ ] Firewall configured
- [ ] Reverse proxy configured with HTTPS
- [ ] Regular security updates planned
- [ ] Backup encryption configured
- [ ] Access logs monitored
- [ ] Sensitive data not in git repository
- [ ] `.env` file in `.gitignore`

### Container Security
- [ ] Using official base images (Alpine)
- [ ] Multi-stage build minimizes attack surface
- [ ] No unnecessary tools in runtime image
- [ ] Health checks configured
- [ ] Resource limits prevent DoS
- [ ] Security options configured (no-new-privileges)

## Monitoring & Maintenance

### Monitoring Setup
- [ ] Container metrics collection: `docker stats`
- [ ] Application logs centralized
- [ ] Health check monitoring
- [ ] Disk space monitoring
- [ ] Database performance monitoring
- [ ] Alert system configured

### Backup Strategy
- [ ] Database backup script: `pg_dump`
- [ ] Download directory backup
- [ ] Configuration backup
- [ ] Backup schedule automated
- [ ] Backup restoration tested
- [ ] Off-site backup configured

### Update Strategy
- [ ] Update procedure documented
- [ ] Rollback procedure documented
- [ ] Backup before updates
- [ ] Test updates in staging first
- [ ] Update window scheduled

## Troubleshooting Verification

### Common Issues Tested
- [ ] Port conflicts handled
- [ ] Database connection retries work
- [ ] Health checks passing
- [ ] Logs accessible and readable
- [ ] Volume permissions correct
- [ ] Network connectivity between containers

### Emergency Procedures
- [ ] Know how to check logs: `docker-compose logs`
- [ ] Know how to restart services: `docker-compose restart`
- [ ] Know how to restore backup
- [ ] Have rollback plan
- [ ] Have emergency contacts

## Documentation

- [ ] Deployment configuration documented
- [ ] Custom changes documented
- [ ] Credentials stored securely (password manager)
- [ ] Network diagram created (if complex setup)
- [ ] Runbook created for operations team
- [ ] Disaster recovery plan documented

## Performance Tuning

- [ ] Resource limits adjusted based on load
- [ ] Database connection pool tuned
- [ ] Download parallelism configured
- [ ] API rate limits considered
- [ ] Caching strategy implemented (if needed)
- [ ] Load testing performed

## Compliance & Best Practices

- [ ] Following Docker best practices
- [ ] Following PostgreSQL best practices
- [ ] Following Go application best practices
- [ ] Secrets not in version control
- [ ] Logging sensitive data avoided
- [ ] GDPR/privacy requirements met (if applicable)
- [ ] Data retention policy defined

## Sign-Off

### Development
- [ ] Developer: _________________ Date: _________
- [ ] Code Review: ________________ Date: _________
- [ ] Testing: ____________________ Date: _________

### Production
- [ ] DevOps Engineer: ____________ Date: _________
- [ ] Security Review: ____________ Date: _________
- [ ] Manager Approval: ___________ Date: _________

## Notes

Use this space to document any deployment-specific notes, issues encountered, or custom configurations:

```
Date: __________
Deployed by: __________
Environment: __________
Version: __________

Notes:




```

---

## Quick Reference Commands

```bash
# Start services
docker-compose up -d

# View logs
docker-compose logs -f stalkeer

# Check health
curl http://localhost:8080/health

# Restart service
docker-compose restart stalkeer

# Stop all services
docker-compose down

# Backup database
docker-compose exec postgres pg_dump -U postgres stalkeer > backup-$(date +%Y%m%d).sql

# View container stats
docker stats stalkeer-app

# Access container shell
docker-compose exec stalkeer sh

# Update and restart
git pull
docker-compose build
docker-compose up -d
```
