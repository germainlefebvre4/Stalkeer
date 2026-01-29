package models

import (
	"testing"
	"time"
)

func TestProcessedLine_TableName(t *testing.T) {
	line := ProcessedLine{}
	expected := "processed_lines"
	if line.TableName() != expected {
		t.Errorf("expected table name %s, got %s", expected, line.TableName())
	}
}

func TestMovie_TableName(t *testing.T) {
	movie := Movie{}
	expected := "movies"
	if movie.TableName() != expected {
		t.Errorf("expected table name %s, got %s", expected, movie.TableName())
	}
}

func TestTVShow_TableName(t *testing.T) {
	tvshow := TVShow{}
	expected := "tvshows"
	if tvshow.TableName() != expected {
		t.Errorf("expected table name %s, got %s", expected, tvshow.TableName())
	}
}

func TestContentType_Constants(t *testing.T) {
	tests := []struct {
		contentType ContentType
		expected    string
	}{
		{ContentTypeMovies, "movies"},
		{ContentTypeTVShows, "tvshows"},
		{ContentTypeChannels, "channels"},
		{ContentTypeUncategorized, "uncategorized"},
	}

	for _, tc := range tests {
		if string(tc.contentType) != tc.expected {
			t.Errorf("expected %s, got %s", tc.expected, tc.contentType)
		}
	}
}

func TestProcessingState_Constants(t *testing.T) {
	tests := []struct {
		state    ProcessingState
		expected string
	}{
		{StateProcessed, "processed"},
		{StatePending, "pending"},
		{StateDownloading, "downloading"},
		{StateDownloaded, "downloaded"},
		{StateFailed, "failed"},
	}

	for _, tc := range tests {
		if string(tc.state) != tc.expected {
			t.Errorf("expected %s, got %s", tc.expected, tc.state)
		}
	}
}

func TestProcessedLine_Creation(t *testing.T) {
	lineURL := "http://example.com/stream"
	now := time.Now()

	line := ProcessedLine{
		LineContent: "#EXTINF:-1 tvg-name=\"Test Movie\" group-title=\"Movies\",Test Movie",
		LineURL:     &lineURL,
		LineHash:    "abc123",
		TvgName:     "Test Movie",
		GroupTitle:  "Movies",
		ProcessedAt: now,
		ContentType: ContentTypeMovies,
		State:       StateProcessed,
		CreatedAt:   now,
		UpdatedAt:   now,
	}

	if line.TvgName != "Test Movie" {
		t.Errorf("expected TvgName 'Test Movie', got %s", line.TvgName)
	}
	if line.ContentType != ContentTypeMovies {
		t.Errorf("expected ContentType movies, got %s", line.ContentType)
	}
	if line.State != StateProcessed {
		t.Errorf("expected State processed, got %s", line.State)
	}
}

func TestMovie_Creation(t *testing.T) {
	genres := "[\"Action\", \"Thriller\"]"
	duration := 120
	now := time.Now()

	movie := Movie{
		TMDBID:     12345,
		TMDBTitle:  "Test Movie",
		TMDBYear:   2024,
		TMDBGenres: &genres,
		Duration:   &duration,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if movie.TMDBID != 12345 {
		t.Errorf("expected TMDBID 12345, got %d", movie.TMDBID)
	}
	if movie.TMDBTitle != "Test Movie" {
		t.Errorf("expected TMDBTitle 'Test Movie', got %s", movie.TMDBTitle)
	}
	if *movie.Duration != 120 {
		t.Errorf("expected Duration 120, got %d", *movie.Duration)
	}
}

func TestTVShow_Creation(t *testing.T) {
	genres := "[\"Drama\", \"Comedy\"]"
	season := 1
	episode := 5
	now := time.Now()

	tvshow := TVShow{
		TMDBID:     67890,
		TMDBTitle:  "Test Show",
		TMDBYear:   2024,
		TMDBGenres: &genres,
		Season:     &season,
		Episode:    &episode,
		CreatedAt:  now,
		UpdatedAt:  now,
	}

	if tvshow.TMDBID != 67890 {
		t.Errorf("expected TMDBID 67890, got %d", tvshow.TMDBID)
	}
	if *tvshow.Season != 1 {
		t.Errorf("expected Season 1, got %d", *tvshow.Season)
	}
	if *tvshow.Episode != 5 {
		t.Errorf("expected Episode 5, got %d", *tvshow.Episode)
	}
}
