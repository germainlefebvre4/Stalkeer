---
description: This file contains instructions for the backend implementation of the project.
applyTo: **/backend/**, **/cmd/**, **/internal/**
---

# Stalkeer Backend Instructions

## Libraries and Dependencies

- Use gorm for database interactions
- Use JSON marshaling for storing filter arrays in database text fields
- Use cobra for CLI structure
- Use viper for configuration management
- Use gin/tonic for REST API implementation

## Folder Structure

- `/internal`: Contains the source code for the application logic.
- `/cmd`: Contains the main application entry point.
- `/docs`: Contains documentation for the project, including API specifications and user guides.

## Configuration

Create a configuration file `config.yml` in the root directory of the project.

- The config file contains the following settings:
  - database connection settings
  - m3u file path
  - filtering options (include/exclude patterns for `group-title` and `tvg-name`)
  - logging settings (level, format)
  - Directories for
    - Downloaded items
    - M3U playlist file

## Unit Tests

- Write unit tests for all functions and methods. Ensure that the tests cover edge cases and error handling.
- Query the database to verify that the expected data is being stored and retrieved correctly.
- Store the tests in the package directory where the functions are defined. Do not create a separate `tests` directory and do not store tests in the root directory.
- Exclude the `postgres/` directory in tests.

## Ignore Files and Directories

- Ignore the following files and directories:
  - `m3u_playlist/m3u_filter.sh`
  - `m3u_playlist/tmp`
  - `m3u_playlist/tv_channels_*`

# Build

- Build the binary in directory `bin/`.
- Run the following command in the root dir to build the application: go build -o bin/stalkeer cmd/main.go

## Core Features

1. **M3U Processing**
   - Parse M3U files and extract metadata (`tvg-name`, `group-title`, `tvg-logo`, stream URL)
   - Track processed lines to avoid duplicates (stored with `created_at`, `updated_at`, `override_by`, `override_at`)
   - Efficiently handle large M3U files with streaming parser

2. **Content Classification**
   - Automatic video type detection:
     - movies (movies)
     - series (tvshows)
     - any other type like live tv or podcast (uncategorized)
   - Season/episode extraction from titles using regex patterns
   - Resolution detection: based on title keywords (4K, 1080p, 720p, etc.) or null if not found

3. **Filtering System**
   - File-based filters defined in `config.yml` under `filters` section
   - Runtime filters managed via REST API (stored in PostgreSQL `filter_configs` table)
   - Runtime filters override file-based filters
   - Filters persist across application restarts (loaded from database on startup)
   - Support for include/exclude patterns with regex on `group-title` and `tvg-name`
   - API endpoints: `PUT/PATCH /api/v1/filters`, `DELETE /api/v1/filters/runtime`

4. **Dry-Run Mode**
   - Test processing without database changes
   - Limited to 100 items for quick analysis
   - Intelligent content analysis to identify:
     - Unclassified content
     - Missing season/episode info
     - TMDB mapping issues
   - Logs actions that would be taken

5. **REST APIs**
   - RESTful JSON API for content management (`/api/v1/*`)
   - CORS support for frontend integration
   - Pagination, sorting, and filtering on all endpoints

## CLI Structure

The application uses Cobra CLI with the following subcommands:
