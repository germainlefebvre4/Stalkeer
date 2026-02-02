package downloader

import (
	"path/filepath"
	"testing"
)

func TestBuildMovieBasePath(t *testing.T) {
	base := "/movies"
	path := buildMovieBasePath(base, "The/Matrix", 1999)
	expectedDir := "The_Matrix (1999)"
	expected := filepath.Join(base, expectedDir, expectedDir)
	if path != expected {
		t.Fatalf("expected %s, got %s", expected, path)
	}
}

func TestBuildTVShowBasePath(t *testing.T) {
	base := "/tvshows"
	path := buildTVShowBasePath(base, "Breaking:Bad", 2008, 1, 2)
	expected := filepath.Join(base, "Breaking_Bad (2008)", "Season 01", "Breaking_Bad (2008) - S01E02")
	if path != expected {
		t.Fatalf("expected %s, got %s", expected, path)
	}
}

func TestSanitizeFilename(t *testing.T) {
	sanitized := sanitizeFilename("Bad/Name:Test?")
	expected := "Bad_Name_Test_"
	if sanitized != expected {
		t.Fatalf("expected %s, got %s", expected, sanitized)
	}
}
