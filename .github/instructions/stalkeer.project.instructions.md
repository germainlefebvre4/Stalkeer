---
description: 'Instructions for the Stalkeer backend project.'
applyTo: **
---

# Stalkeer Project Instructions

Stalkeer parse Radarr and Sonarr items and download missing items from a m3u playlist via direct links.

## Application

### Purpose

- Read a m3u_playlist file and store the movies/tvshows (exclude channels and podcasts) information in a PostgreSQL database.
- Parse item from m3u playlist and match them against TMDB to enrich the data (title, year, poster, overview, genres, etc) and store in the database.
- Provide a REST API to query the movie/tvshows information and allow to filter items based on specific criteria.
- Parse Radarr and Sonarr items to identify missing movies/tvshows.
- Download missing items from the m3u playlist via direct links and store them locally.
- Download the m3u playlist file

### Specifications

## Architecture

- A backend service handles the parsing of the m3u playlist, storing data in the PostgreSQL database, and providing the REST API.
- A database to store the parsed m3u playlist data and track downloaded items.

## Technologies

- Backend in golang
- Database in PostgreSQL

## Applicative Workflows

### Process Workflow

- M3U Parsing Workflow: table `processed_lines`
  - Read the m3u file line by line
  - Parse metadata and stream URLs
  - Classify content into movies, tvshows, or uncategorized
  - Store parsed data in PostgreSQL database
- TMDB Enrichment Workflow: table `movies` and `tvshows`
  - For each parsed item, query TMDB API
  - Retrieve additional metadata (title, year, poster, overview, genres)
  - Update database records with enriched data

### Sonarr/Radarr Sync Workflow

- Query Sonarr/Radarr API for existing movies/tvshows
- Compare with database records to identify missing items
- For missing items, find matching entries in the m3u playlist
- Download missing items via direct links and store locally
- Update database records to mark items as downloaded

### M3U Download Workflow

- Download the m3u playlist file (one time) (configurable in `config.yml`)
- Archive and rotate old m3u files to avoid clutter (keep last 5 files)
