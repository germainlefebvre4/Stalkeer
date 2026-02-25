package processor

import (
	"fmt"

	"github.com/glefebvre/stalkeer/internal/external/tmdb"
	"github.com/glefebvre/stalkeer/internal/models"
	"gorm.io/gorm"
)

// EnrichTVDBOptions holds configuration for the TVDB ID backfill operation.
type EnrichTVDBOptions struct {
	DryRun  bool
	Limit   int
	Verbose bool
}

// EnrichTVDBStats holds the results of a TVDB ID backfill run.
type EnrichTVDBStats struct {
	Processed int
	Updated   int
	Skipped   int // record has no TVDB entry on TMDB
	Errors    int
}

// EnrichMissingTVDBIDs fetches all Movie and TVShow records that have a tmdb_id
// but a missing tvdb_id, queries the TMDB External IDs endpoint, and updates
// the database. TVShow records are deduplicated by tmdb_id so the API is called
// only once per unique show.
func EnrichMissingTVDBIDs(db *gorm.DB, client *tmdb.Client, opts EnrichTVDBOptions) (*EnrichTVDBStats, error) {
	stats := &EnrichTVDBStats{}

	if err := enrichMoviesTVDB(db, client, opts, stats); err != nil {
		return stats, err
	}
	if err := enrichTVShowsTVDB(db, client, opts, stats); err != nil {
		return stats, err
	}

	return stats, nil
}

func enrichMoviesTVDB(db *gorm.DB, client *tmdb.Client, opts EnrichTVDBOptions, stats *EnrichTVDBStats) error {
	const batchSize = 100
	offset := 0

	for {
		if opts.Limit > 0 && stats.Processed >= opts.Limit {
			break
		}

		var movies []models.Movie
		if err := db.Where("tvdb_id IS NULL AND tmdb_id != 0").
			Offset(offset).Limit(batchSize).Find(&movies).Error; err != nil {
			return fmt.Errorf("failed to query movies: %w", err)
		}
		if len(movies) == 0 {
			break
		}

		for i := range movies {
			if opts.Limit > 0 && stats.Processed >= opts.Limit {
				break
			}
			stats.Processed++
			movie := &movies[i]

			if opts.DryRun {
				fmt.Printf("  [dry-run] movie: %s (%d) tmdb_id=%d\n", movie.TMDBTitle, movie.TMDBYear, movie.TMDBID)
				continue
			}

			if opts.Verbose {
				fmt.Printf("  [movie] Fetching external IDs for %s tmdb_id=%d\n", movie.TMDBTitle, movie.TMDBID)
			}

			ext, err := client.GetMovieExternalIDs(movie.TMDBID)
			if err != nil {
				stats.Errors++
				fmt.Printf("  [warn] Failed to fetch external IDs for movie tmdb_id=%d: %v\n", movie.TMDBID, err)
				continue
			}

			if ext.TVDBID == nil {
				stats.Skipped++
				if opts.Verbose {
					fmt.Printf("  [skip] No TVDB entry for movie tmdb_id=%d\n", movie.TMDBID)
				}
				continue
			}

			if err := db.Model(movie).Update("tvdb_id", ext.TVDBID).Error; err != nil {
				stats.Errors++
				fmt.Printf("  [warn] Failed to update movie id=%d: %v\n", movie.ID, err)
				continue
			}
			stats.Updated++
			if opts.Verbose {
				fmt.Printf("  [updated] Movie %s: tvdb_id=%d\n", movie.TMDBTitle, *ext.TVDBID)
			}
		}

		offset += batchSize
	}

	return nil
}

func enrichTVShowsTVDB(db *gorm.DB, client *tmdb.Client, opts EnrichTVDBOptions, stats *EnrichTVDBStats) error {
	const batchSize = 100
	offset := 0

	// Cache TMDB ID → TVDB ID to avoid redundant API calls across rows sharing the same show.
	// nil value means the show has no TVDB entry (or the call errored); sentinel allows distinguishing
	// "not yet queried" (absent from map) from "queried, no result" (nil in map).
	type cacheEntry struct {
		tvdbID  *int
		hadErr  bool
	}
	cache := make(map[int]cacheEntry)

	for {
		if opts.Limit > 0 && stats.Processed >= opts.Limit {
			break
		}

		var shows []models.TVShow
		if err := db.Where("tvdb_id IS NULL AND tmdb_id != 0").
			Offset(offset).Limit(batchSize).Find(&shows).Error; err != nil {
			return fmt.Errorf("failed to query tvshows: %w", err)
		}
		if len(shows) == 0 {
			break
		}

		for i := range shows {
			if opts.Limit > 0 && stats.Processed >= opts.Limit {
				break
			}
			stats.Processed++
			show := &shows[i]

			if opts.DryRun {
				fmt.Printf("  [dry-run] tvshow: %s tmdb_id=%d\n", show.TMDBTitle, show.TMDBID)
				continue
			}

			if opts.Verbose {
				fmt.Printf("  [tvshow] %s tmdb_id=%d\n", show.TMDBTitle, show.TMDBID)
			}

			// Resolve TVDB ID — use cache to deduplicate API calls per unique tmdb_id.
			entry, cached := cache[show.TMDBID]
			if !cached {
				ext, err := client.GetTVShowExternalIDs(show.TMDBID)
				if err != nil {
					stats.Errors++
					fmt.Printf("  [warn] Failed to fetch external IDs for tvshow tmdb_id=%d: %v\n", show.TMDBID, err)
					cache[show.TMDBID] = cacheEntry{hadErr: true}
					continue
				}
				entry = cacheEntry{tvdbID: ext.TVDBID}
				cache[show.TMDBID] = entry
			}

			if entry.hadErr {
				stats.Errors++
				continue
			}
			if entry.tvdbID == nil {
				stats.Skipped++
				if opts.Verbose {
					fmt.Printf("  [skip] No TVDB entry for tvshow tmdb_id=%d\n", show.TMDBID)
				}
				continue
			}

			if err := db.Model(show).Update("tvdb_id", entry.tvdbID).Error; err != nil {
				stats.Errors++
				fmt.Printf("  [warn] Failed to update tvshow id=%d: %v\n", show.ID, err)
				continue
			}
			stats.Updated++
			if opts.Verbose {
				fmt.Printf("  [updated] TVShow %s: tvdb_id=%d\n", show.TMDBTitle, *entry.tvdbID)
			}
		}

		offset += batchSize
	}

	return nil
}
