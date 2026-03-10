package main

import (
	"fmt"
	"path/filepath"
)

// formatBytes converts a byte count to a human-readable string (e.g. "1.23 MB").
func formatBytes(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for n := bytes / unit; n >= unit; n /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.2f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}

// sanitizeFilename replaces characters that are invalid on common filesystems with underscores.
func sanitizeFilename(name string) string {
	replacer := map[rune]rune{
		'/':  '_',
		'\\': '_',
		':':  '_',
		'*':  '_',
		'?':  '_',
		'"':  '_',
		'<':  '_',
		'>':  '_',
		'|':  '_',
	}
	result := []rune(name)
	for i, r := range result {
		if replacement, ok := replacer[r]; ok {
			result[i] = replacement
		}
	}
	return string(result)
}

// valueOrEmpty returns the dereferenced string or an empty string if the pointer is nil.
func valueOrEmpty(ptr *string) string {
	if ptr == nil {
		return ""
	}
	return *ptr
}

// buildSonarrDestPath constructs the base destination path for a TV show episode download.
// It uses seriesPath (from the Sonarr API) as the authoritative root directory, which
// already encodes the correct Sonarr root folder. When seriesPath is empty it falls back
// to joining fallbackBase with a sanitised seriesTitle.
// The second return value is true when the fallback was used.
func buildSonarrDestPath(seriesPath, fallbackBase, seriesTitle string, seasonNum, episodeNum int) (string, bool) {
	root := seriesPath
	usedFallback := false
	if root == "" {
		root = filepath.Join(fallbackBase, sanitizeFilename(seriesTitle))
		usedFallback = true
	}
	return filepath.Join(
		root,
		fmt.Sprintf("Season %02d", seasonNum),
		fmt.Sprintf("%s - S%02dE%02d", sanitizeFilename(seriesTitle), seasonNum, episodeNum),
	), usedFallback
}

// buildRadarrDestPath constructs the base destination path for a movie download.
// It uses moviePath (from the Radarr API) as the authoritative root directory.
// When moviePath is empty it falls back to joining fallbackBase with the standard
// movie directory name.
// The second return value is true when the fallback was used.
func buildRadarrDestPath(moviePath, fallbackBase, movieTitle string, movieYear int) (string, bool) {
	fileBase := fmt.Sprintf("%s (%d)", sanitizeFilename(movieTitle), movieYear)
	root := moviePath
	usedFallback := false
	if root == "" {
		root = filepath.Join(fallbackBase, fileBase)
		usedFallback = true
	}
	return filepath.Join(root, fileBase), usedFallback
}
