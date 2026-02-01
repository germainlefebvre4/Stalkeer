package downloader

import (
	"fmt"
	"path/filepath"
)

func buildMovieBasePath(basePath, title string, year int) string {
	dir := fmt.Sprintf("%s (%d)", sanitizeFilename(title), year)
	return filepath.Join(basePath, dir, dir)
}

func buildTVShowBasePath(basePath, seriesTitle string, season, episode int) string {
	seriesDir := sanitizeFilename(seriesTitle)
	seasonDir := fmt.Sprintf("Season %02d", season)
	fileName := fmt.Sprintf("%s - S%02dE%02d", sanitizeFilename(seriesTitle), season, episode)
	return filepath.Join(basePath, seriesDir, seasonDir, fileName)
}

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
