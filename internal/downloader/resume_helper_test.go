package downloader

import (
	"testing"

	"github.com/glefebvre/stalkeer/internal/models"
)

func TestNormalizeContentType(t *testing.T) {
	cases := []struct {
		input    string
		expected string
	}{
		{"movies", string(models.ContentTypeMovies)},
		{"radarr", string(models.ContentTypeMovies)},
		{"tvshows", string(models.ContentTypeTVShows)},
		{"sonarr", string(models.ContentTypeTVShows)},
		{"unknown", ""},
	}

	for _, tc := range cases {
		if got := normalizeContentType(tc.input); got != tc.expected {
			t.Fatalf("expected %q, got %q for input %q", tc.expected, got, tc.input)
		}
	}
}

func TestHasContentType(t *testing.T) {
	download := &models.DownloadInfo{
		ProcessedLines: []models.ProcessedLine{
			{ContentType: models.ContentTypeMovies},
			{ContentType: models.ContentTypeTVShows},
		},
	}

	if !hasContentType(download, string(models.ContentTypeTVShows)) {
		t.Fatalf("expected tvshows content type to match")
	}

	if hasContentType(download, "channels") {
		t.Fatalf("expected channels content type to not match")
	}
}
