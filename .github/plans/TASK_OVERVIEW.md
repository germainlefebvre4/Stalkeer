# Stalkeer Task Overview - Updated

## Phase Organization

### ✅ Phase 1: Foundation (Complete)
- [x] 1.1: Project Structure
- [x] 1.2: Configuration Management
- [x] 1.3: Database Schema & GORM Setup
- [x] 1.4: Unit Test Foundation

### ⏳ Phase 2: Core Functionality (In Progress)
- [ ] 2.1: M3U Parser Implementation
- [ ] 2.2: Content Classification Engine
- [ ] 2.3: Filter System
- [ ] 2.4: REST API
- [ ] 2.5: Dry-run Mode
- [ ] 2.6: CLI Structure
- [ ] **2.7: TMDB Integration & Content Enrichment** ⭐ NEW

### ⏳ Phase 3: Error Handling & Integration
- [ ] 3.1: Error Handling
- [ ] 3.2: Radarr/Sonarr Integration
- [ ] 3.4: Comprehensive Testing

### ⏳ Phase 4: Download Implementation
- [ ] 4.1: Download Implementation
  - Depends on: 2.7 (TMDB Integration)

### ⏳ Phase 5: Performance & Deployment
- [ ] 5.1: Performance Optimization
- [ ] 5.2: Docker Deployment
- [ ] 5.3: CI/CD & Release

### ⏳ Phase 6: Documentation
- [ ] 6.1: Documentation

## Task 2.7 Details

### TMDB Integration & Content Enrichment

**Purpose**: Enrich M3U playlist items with authoritative metadata from The Movie Database (TMDB) API.

**Key Features**:
- TMDB API client with rate limiting and caching
- Intelligent matching algorithm (title + year)
- Batch processing with progress tracking
- CLI and API integration
- Comprehensive error handling

**Timeline**: 9-13 days (~2 weeks)

**Dependencies**:
- ✅ Task 1.3 (Database Schema)
- ⏳ Task 2.1 (M3U Parser)
- ⏳ Task 2.2 (Content Classification)

**Deliverables**:
1. `/internal/tmdb/` - TMDB API client
2. `/internal/enrichment/` - Enrichment service
3. `stalkeer enrich` CLI command
4. REST API endpoints for enrichment
5. Configuration support for TMDB API key
6. Comprehensive tests (unit, integration, performance)

**Configuration Example**:
```yaml
tmdb:
  api_key: "your_api_key"
  rate_limit:
    requests_per_10s: 40
  cache:
    enabled: true
    ttl: 24h
```

**CLI Usage**:
```bash
# Enrich all items
stalkeer enrich

# Enrich only movies
stalkeer enrich --content-type movies

# Dry run
stalkeer enrich --dry-run

# Force re-enrichment
stalkeer enrich --force
```

## Task Relationships

```
Phase 1 (Complete)
  └─> Phase 2
       ├─> 2.1 M3U Parser
       ├─> 2.2 Content Classification
       │    └─> 2.7 TMDB Integration ⭐
       ├─> 2.3 Filter System
       ├─> 2.4 REST API
       ├─> 2.5 Dry-run Mode
       └─> 2.6 CLI Structure

Phase 2
  └─> Phase 3
       ├─> 3.1 Error Handling
       ├─> 3.2 Radarr/Sonarr Integration
       │    └─> Phase 4
       │         └─> 4.1 Download (uses 2.7) ⭐
       └─> 3.4 Testing

Phase 4
  └─> Phase 5
       ├─> 5.1 Performance
       ├─> 5.2 Docker
       └─> 5.3 CI/CD

Phase 5
  └─> Phase 6
       └─> 6.1 Documentation
```

## Quick Facts

- **Total Tasks**: 18
- **Completed**: 4 (Phase 1)
- **Remaining**: 14
- **New in This Update**: 1 (Task 2.7)
- **Current Phase**: Phase 2 (Core Functionality)

## Priority Tasks for Next Sprint

1. **Task 2.1**: M3U Parser Implementation (prerequisite for 2.7)
2. **Task 2.2**: Content Classification Engine (prerequisite for 2.7)
3. **Task 2.7**: TMDB Integration ⭐ (prerequisite for 4.1)

## Getting TMDB API Key

1. Create account at https://www.themoviedb.org/
2. Go to Settings > API
3. Request API key (free for non-commercial)
4. Add to config: `STALKEER_TMDB_API_KEY` or `config.yml`

## Resources

- **Task Files**: `.github/plans/tasks/`
- **Implementation Status**: `docs/STATUS.md`
- **Database Schema**: `docs/DATABASE.md`
- **TMDB Update Summary**: `.github/plans/TMDB_INTEGRATION_UPDATE.md`
- **TMDB API Docs**: https://developers.themoviedb.org/3

---

**Last Updated**: January 29, 2026  
**Version**: 0.1.0  
**Current Status**: Phase 1 Complete, Phase 2 Planning
