---
description: 'Instructions for the Stalkeer backend project.'
applyTo: **
---

# Stalkeer Project Instructions

Stalkeer parse Radarr and Sonarr items and download missing items from a m3u playlist via direct links.

## Application

### Purpose

- Read a m3u_playlist file and store the movies/tvshows (exclude channels and podcasts) information in a PostgreSQL database.
- Provide a REST API to query the movie/tvshows information and allow to filter items based on specific criteria.
- Parse Radarr and Sonarr items to identify missing movies/tvshows.
- Download missing items from the m3u playlist via direct links and store them locally.

### Specifications

## Architecture

- A backend service handles the parsing of the m3u playlist, storing data in the PostgreSQL database, and providing the REST API.
- A database to store the parsed m3u playlist data and track downloaded items.

## Technologies

- Backend in golang
- Database in PostgreSQL
