package classifier

import (
	"testing"
)

func TestExtractSeasonEpisode(t *testing.T) {
	c := New()

	tests := []struct {
		name            string
		title           string
		expectedSeason  *int
		expectedEpisode *int
	}{
		{
			name:            "Standard S01E05",
			title:           "Show Name S01E05 720p",
			expectedSeason:  intPtr(1),
			expectedEpisode: intPtr(5),
		},
		{
			name:            "Standard S1E5",
			title:           "Show Name S1E5",
			expectedSeason:  intPtr(1),
			expectedEpisode: intPtr(5),
		},
		{
			name:            "Lowercase s01e05",
			title:           "show name s01e05",
			expectedSeason:  intPtr(1),
			expectedEpisode: intPtr(5),
		},
		{
			name:            "Alternative 1x05",
			title:           "Show Name 1x05",
			expectedSeason:  intPtr(1),
			expectedEpisode: intPtr(5),
		},
		{
			name:            "Alternative 01x05",
			title:           "Show Name 01x05",
			expectedSeason:  intPtr(1),
			expectedEpisode: intPtr(5),
		},
		{
			name:            "Words Season 1 Episode 5",
			title:           "Show Name Season 1 Episode 5",
			expectedSeason:  intPtr(1),
			expectedEpisode: intPtr(5),
		},
		{
			name:            "Words Season 01 Episode 05",
			title:           "Show Name Season 01 Episode 05",
			expectedSeason:  intPtr(1),
			expectedEpisode: intPtr(5),
		},
		{
			name:            "French Saison 1 Episode 5",
			title:           "Nom du Show Saison 1 Episode 5",
			expectedSeason:  intPtr(1),
			expectedEpisode: intPtr(5),
		},
		{
			name:            "Spanish Temporada 1 Episodio 5",
			title:           "Nombre del Show Temporada 1 Episodio 5",
			expectedSeason:  intPtr(1),
			expectedEpisode: intPtr(5),
		},
		{
			name:            "German Staffel 1 Folge 5",
			title:           "Show Name Staffel 1 Folge 5",
			expectedSeason:  intPtr(1),
			expectedEpisode: intPtr(5),
		},
		{
			name:            "Compact s1e5",
			title:           "Show Name s1e5",
			expectedSeason:  intPtr(1),
			expectedEpisode: intPtr(5),
		},
		{
			name:            "Three digit episode S01E123",
			title:           "Show Name S01E123",
			expectedSeason:  intPtr(1),
			expectedEpisode: intPtr(123),
		},
		{
			name:            "Two digit season S12E05",
			title:           "Show Name S12E05",
			expectedSeason:  intPtr(12),
			expectedEpisode: intPtr(5),
		},
		{
			name:            "No season/episode",
			title:           "Movie Title (2023) 1080p",
			expectedSeason:  nil,
			expectedEpisode: nil,
		},
		{
			name:            "Movie with year",
			title:           "The Matrix (1999)",
			expectedSeason:  nil,
			expectedEpisode: nil,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			season, episode := c.ExtractSeasonEpisode(tt.title)

			if !intPtrEqual(season, tt.expectedSeason) {
				t.Errorf("Season mismatch for '%s': got %v, want %v", tt.title, ptrToString(season), ptrToString(tt.expectedSeason))
			}

			if !intPtrEqual(episode, tt.expectedEpisode) {
				t.Errorf("Episode mismatch for '%s': got %v, want %v", tt.title, ptrToString(episode), ptrToString(tt.expectedEpisode))
			}
		})
	}
}

func TestExtractResolution(t *testing.T) {
	c := New()

	tests := []struct {
		name     string
		title    string
		expected *string
	}{
		{
			name:     "4K",
			title:    "Movie Title 4K",
			expected: strPtr("4K"),
		},
		{
			name:     "UHD",
			title:    "Movie Title UHD",
			expected: strPtr("4K"),
		},
		{
			name:     "2160p",
			title:    "Movie Title 2160p",
			expected: strPtr("4K"),
		},
		{
			name:     "1080p",
			title:    "Movie Title 1080p",
			expected: strPtr("1080p"),
		},
		{
			name:     "FullHD",
			title:    "Movie Title FullHD",
			expected: strPtr("1080p"),
		},
		{
			name:     "FHD",
			title:    "Movie Title FHD",
			expected: strPtr("1080p"),
		},
		{
			name:     "720p",
			title:    "Movie Title 720p",
			expected: strPtr("720p"),
		},
		{
			name:     "HD",
			title:    "Movie Title HD",
			expected: strPtr("720p"),
		},
		{
			name:     "480p",
			title:    "Movie Title 480p",
			expected: strPtr("480p"),
		},
		{
			name:     "SD",
			title:    "Movie Title SD",
			expected: strPtr("480p"),
		},
		{
			name:     "No resolution",
			title:    "Movie Title",
			expected: nil,
		},
		{
			name:     "Lowercase 1080p",
			title:    "movie title 1080p",
			expected: strPtr("1080p"),
		},
		{
			name:     "Mixed case 4k",
			title:    "Movie Title 4k",
			expected: strPtr("4K"),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.ExtractResolution(tt.title)

			if !strPtrEqual(result, tt.expected) {
				t.Errorf("Resolution mismatch for '%s': got %v, want %v", tt.title, ptrToString(result), ptrToString(tt.expected))
			}
		})
	}
}

func TestClassify(t *testing.T) {
	c := New()

	tests := []struct {
		name               string
		title              string
		groupTitle         string
		expectedType       ContentType
		expectedSeason     *int
		expectedEpisode    *int
		expectedResolution *string
		minConfidence      int
	}{
		{
			name:               "Clear series with S01E05",
			title:              "Breaking Bad S01E05 1080p",
			groupTitle:         "",
			expectedType:       ContentTypeSeries,
			expectedSeason:     intPtr(1),
			expectedEpisode:    intPtr(5),
			expectedResolution: strPtr("1080p"),
			minConfidence:      80,
		},
		{
			name:               "Series with 1x05 format",
			title:              "Game of Thrones 1x05 720p",
			groupTitle:         "",
			expectedType:       ContentTypeSeries,
			expectedSeason:     intPtr(1),
			expectedEpisode:    intPtr(5),
			expectedResolution: strPtr("720p"),
			minConfidence:      80,
		},
		{
			name:               "Movie with year",
			title:              "The Matrix (1999) 1080p",
			groupTitle:         "",
			expectedType:       ContentTypeMovie,
			expectedSeason:     nil,
			expectedEpisode:    nil,
			expectedResolution: strPtr("1080p"),
			minConfidence:      50,
		},
		{
			name:               "Movie with year and 4K",
			title:              "Inception (2010) 4K",
			groupTitle:         "",
			expectedType:       ContentTypeMovie,
			expectedSeason:     nil,
			expectedEpisode:    nil,
			expectedResolution: strPtr("4K"),
			minConfidence:      50,
		},
		{
			name:               "French series",
			title:              "Les Revenants Saison 1 Episode 3 720p",
			groupTitle:         "",
			expectedType:       ContentTypeSeries,
			expectedSeason:     intPtr(1),
			expectedEpisode:    intPtr(3),
			expectedResolution: strPtr("720p"),
			minConfidence:      80,
		},
		{
			name:               "Spanish series",
			title:              "La Casa de Papel Temporada 2 Episodio 7 1080p",
			groupTitle:         "",
			expectedType:       ContentTypeSeries,
			expectedSeason:     intPtr(2),
			expectedEpisode:    intPtr(7),
			expectedResolution: strPtr("1080p"),
			minConfidence:      80,
		},
		{
			name:               "German series",
			title:              "Dark Staffel 1 Folge 5 UHD",
			groupTitle:         "",
			expectedType:       ContentTypeSeries,
			expectedSeason:     intPtr(1),
			expectedEpisode:    intPtr(5),
			expectedResolution: strPtr("4K"),
			minConfidence:      80,
		},
		{
			name:               "Series keyword without pattern",
			title:              "My Series Name 720p",
			groupTitle:         "",
			expectedType:       ContentTypeSeries,
			expectedSeason:     nil,
			expectedEpisode:    nil,
			expectedResolution: strPtr("720p"),
			minConfidence:      30,
		},
		{
			name:               "Movie keyword",
			title:              "Best Movie Ever 1080p",
			groupTitle:         "",
			expectedType:       ContentTypeMovie,
			expectedSeason:     nil,
			expectedEpisode:    nil,
			expectedResolution: strPtr("1080p"),
			minConfidence:      40,
		},
		{
			name:               "Uncategorized - no clear indicators",
			title:              "Random Content Name",
			groupTitle:         "",
			expectedType:       ContentTypeUncategorized,
			expectedSeason:     nil,
			expectedEpisode:    nil,
			expectedResolution: nil,
			minConfidence:      0,
		},
		{
			name:               "Series detected from group title",
			title:              "Hunter with a scalpel S01 E01",
			groupTitle:         "Séries K-DRAMA",
			expectedType:       ContentTypeSeries,
			expectedSeason:     intPtr(1),
			expectedEpisode:    intPtr(1),
			expectedResolution: nil,
			minConfidence:      70,
		},
		{
			name:               "Movie detected from group title FR",
			title:              "Clifford (FHD VOSTFR)",
			groupTitle:         "FR: FILMS - Disney+",
			expectedType:       ContentTypeMovie,
			expectedSeason:     nil,
			expectedEpisode:    nil,
			expectedResolution: strPtr("1080p"),
			minConfidence:      70,
		},
		{
			name:               "Movie detected from group title ES",
			title:              "Avengers (2012) (ES)",
			groupTitle:         "ES:Movies - películas",
			expectedType:       ContentTypeMovie,
			expectedSeason:     nil,
			expectedEpisode:    nil,
			expectedResolution: nil,
			minConfidence:      70,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.Classify(tt.title, tt.groupTitle)

			if result.ContentType != tt.expectedType {
				t.Errorf("Content type mismatch for '%s': got %v, want %v", tt.title, result.ContentType, tt.expectedType)
			}

			if !intPtrEqual(result.Season, tt.expectedSeason) {
				t.Errorf("Season mismatch for '%s': got %v, want %v", tt.title, ptrToString(result.Season), ptrToString(tt.expectedSeason))
			}

			if !intPtrEqual(result.Episode, tt.expectedEpisode) {
				t.Errorf("Episode mismatch for '%s': got %v, want %v", tt.title, ptrToString(result.Episode), ptrToString(tt.expectedEpisode))
			}

			if !strPtrEqual(result.Resolution, tt.expectedResolution) {
				t.Errorf("Resolution mismatch for '%s': got %v, want %v", tt.title, ptrToString(result.Resolution), ptrToString(tt.expectedResolution))
			}

			if result.Confidence < tt.minConfidence {
				t.Errorf("Confidence too low for '%s': got %d, want at least %d", tt.title, result.Confidence, tt.minConfidence)
			}
		})
	}
}

func TestClassifyEdgeCases(t *testing.T) {
	c := New()

	tests := []struct {
		name         string
		title        string
		groupTitle   string
		expectedType ContentType
	}{
		{
			name:         "Empty string",
			title:        "",
			groupTitle:   "",
			expectedType: ContentTypeUncategorized,
		},
		{
			name:         "Only numbers",
			title:        "12345",
			groupTitle:   "",
			expectedType: ContentTypeUncategorized,
		},
		{
			name:         "Malformed season/episode S0E0",
			title:        "Show S0E0",
			groupTitle:   "",
			expectedType: ContentTypeSeries,
		},
		{
			name:         "Year only (2023)",
			title:        "(2023)",
			groupTitle:   "",
			expectedType: ContentTypeMovie,
		},
		{
			name:         "Multiple years",
			title:        "Show (2020) (2021)",
			groupTitle:   "",
			expectedType: ContentTypeMovie,
		},
		{
			name:         "Year in middle",
			title:        "Show 2023 Name",
			groupTitle:   "",
			expectedType: ContentTypeUncategorized,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := c.Classify(tt.title, tt.groupTitle)

			if result.ContentType != tt.expectedType {
				t.Errorf("Content type mismatch for '%s': got %v, want %v", tt.title, result.ContentType, tt.expectedType)
			}
		})
	}
}

func BenchmarkClassify(b *testing.B) {
	c := New()
	titles := []string{
		"Breaking Bad S01E05 1080p",
		"The Matrix (1999) 4K",
		"Game of Thrones 1x05 720p",
		"Inception (2010) UHD",
		"Random Content Name",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		c.Classify(titles[i%len(titles)], "")
	}
}

func BenchmarkClassify10k(b *testing.B) {
	c := New()
	title := "Breaking Bad S01E05 1080p"

	b.ResetTimer()
	for i := 0; i < 10000; i++ {
		c.Classify(title, "")
	}
}

// Helper functions

func intPtr(i int) *int {
	return &i
}

func strPtr(s string) *string {
	return &s
}

func intPtrEqual(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func strPtrEqual(a, b *string) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func ptrToString(v interface{}) string {
	if v == nil {
		return "nil"
	}
	switch val := v.(type) {
	case *int:
		if val == nil {
			return "nil"
		}
		return string(rune(*val + '0'))
	case *string:
		if val == nil {
			return "nil"
		}
		return *val
	default:
		return "unknown"
	}
}
