package filter

import (
	"testing"

	"github.com/glefebvre/stalkeer/internal/models"
)

func TestValidatePattern(t *testing.T) {
	tests := []struct {
		name    string
		pattern string
		wantErr bool
	}{
		{
			name:    "Valid simple pattern",
			pattern: "^Movies",
			wantErr: false,
		},
		{
			name:    "Valid complex pattern",
			pattern: "^(Movies|TV Shows).*HD$",
			wantErr: false,
		},
		{
			name:    "Invalid pattern - unclosed group",
			pattern: "^(Movies",
			wantErr: true,
		},
		{
			name:    "Invalid pattern - bad escape",
			pattern: "\\k",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidatePattern(tt.pattern)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidatePattern() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestManager_Matches(t *testing.T) {
	tests := []struct {
		name            string
		includePatterns []string
		excludePatterns []string
		attribute       string
		value           string
		want            bool
	}{
		{
			name:            "No filters - allow all",
			includePatterns: []string{},
			excludePatterns: []string{},
			attribute:       "group_title",
			value:           "Movies HD",
			want:            true,
		},
		{
			name:            "Include pattern matches",
			includePatterns: []string{"^Movies"},
			excludePatterns: []string{},
			attribute:       "group_title",
			value:           "Movies HD",
			want:            true,
		},
		{
			name:            "Include pattern doesn't match",
			includePatterns: []string{"^TV Shows"},
			excludePatterns: []string{},
			attribute:       "group_title",
			value:           "Movies HD",
			want:            false,
		},
		{
			name:            "Exclude pattern matches",
			includePatterns: []string{},
			excludePatterns: []string{"XXX"},
			attribute:       "group_title",
			value:           "Movies XXX",
			want:            false,
		},
		{
			name:            "Exclude pattern doesn't match",
			includePatterns: []string{},
			excludePatterns: []string{"XXX"},
			attribute:       "group_title",
			value:           "Movies HD",
			want:            true,
		},
		{
			name:            "Include and exclude - include matches, exclude doesn't",
			includePatterns: []string{"^Movies"},
			excludePatterns: []string{"XXX"},
			attribute:       "group_title",
			value:           "Movies HD",
			want:            true,
		},
		{
			name:            "Include and exclude - both match (exclude wins)",
			includePatterns: []string{"^Movies"},
			excludePatterns: []string{"XXX"},
			attribute:       "group_title",
			value:           "Movies XXX",
			want:            false,
		},
		{
			name:            "Multiple include patterns - one matches",
			includePatterns: []string{"^TV Shows", "^Movies"},
			excludePatterns: []string{},
			attribute:       "group_title",
			value:           "Movies HD",
			want:            true,
		},
		{
			name:            "Multiple exclude patterns - one matches",
			includePatterns: []string{},
			excludePatterns: []string{"XXX", "Adult"},
			attribute:       "group_title",
			value:           "Movies Adult",
			want:            false,
		},
		{
			name:            "Case sensitive matching",
			includePatterns: []string{"^movies"},
			excludePatterns: []string{},
			attribute:       "group_title",
			value:           "Movies HD",
			want:            false,
		},
		{
			name:            "Case insensitive pattern",
			includePatterns: []string{"(?i)^movies"},
			excludePatterns: []string{},
			attribute:       "group_title",
			value:           "Movies HD",
			want:            true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager()
			if err := m.loadFilterSet(tt.attribute, tt.includePatterns, tt.excludePatterns, false); err != nil {
				t.Fatalf("Failed to load filter set: %v", err)
			}

			got := m.Matches(tt.attribute, tt.value)
			if got != tt.want {
				t.Errorf("Matches() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestManager_MatchesItem(t *testing.T) {
	tests := []struct {
		name              string
		groupTitleInclude []string
		groupTitleExclude []string
		tvgNameInclude    []string
		tvgNameExclude    []string
		item              models.ProcessedLine
		want              bool
	}{
		{
			name:              "No filters - allow all",
			groupTitleInclude: []string{},
			groupTitleExclude: []string{},
			tvgNameInclude:    []string{},
			tvgNameExclude:    []string{},
			item: models.ProcessedLine{
				GroupTitle: "Movies HD",
				TvgName:    "The Matrix",
			},
			want: true,
		},
		{
			name:              "Group title filter matches",
			groupTitleInclude: []string{"^Movies"},
			groupTitleExclude: []string{},
			tvgNameInclude:    []string{},
			tvgNameExclude:    []string{},
			item: models.ProcessedLine{
				GroupTitle: "Movies HD",
				TvgName:    "The Matrix",
			},
			want: true,
		},
		{
			name:              "Group title filter doesn't match",
			groupTitleInclude: []string{"^TV Shows"},
			groupTitleExclude: []string{},
			tvgNameInclude:    []string{},
			tvgNameExclude:    []string{},
			item: models.ProcessedLine{
				GroupTitle: "Movies HD",
				TvgName:    "The Matrix",
			},
			want: false,
		},
		{
			name:              "Tvg name filter matches",
			groupTitleInclude: []string{},
			groupTitleExclude: []string{},
			tvgNameInclude:    []string{"Matrix"},
			tvgNameExclude:    []string{},
			item: models.ProcessedLine{
				GroupTitle: "Movies HD",
				TvgName:    "The Matrix",
			},
			want: true,
		},
		{
			name:              "Both filters match",
			groupTitleInclude: []string{"^Movies"},
			groupTitleExclude: []string{},
			tvgNameInclude:    []string{"Matrix"},
			tvgNameExclude:    []string{},
			item: models.ProcessedLine{
				GroupTitle: "Movies HD",
				TvgName:    "The Matrix",
			},
			want: true,
		},
		{
			name:              "Group matches but tvg name excluded",
			groupTitleInclude: []string{"^Movies"},
			groupTitleExclude: []string{},
			tvgNameInclude:    []string{},
			tvgNameExclude:    []string{"Trailer"},
			item: models.ProcessedLine{
				GroupTitle: "Movies HD",
				TvgName:    "The Matrix Trailer",
			},
			want: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			m := NewManager()

			if err := m.loadFilterSet("group_title", tt.groupTitleInclude, tt.groupTitleExclude, false); err != nil {
				t.Fatalf("Failed to load group_title filter: %v", err)
			}

			if err := m.loadFilterSet("tvg_name", tt.tvgNameInclude, tt.tvgNameExclude, false); err != nil {
				t.Fatalf("Failed to load tvg_name filter: %v", err)
			}

			got := m.MatchesItem(tt.item)
			if got != tt.want {
				t.Errorf("MatchesItem() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestManager_RuntimeFilterPrecedence(t *testing.T) {
	m := NewManager()

	// Add config-based filter that includes "Movies"
	if err := m.loadFilterSet("group_title", []string{"^Movies"}, []string{}, false); err != nil {
		t.Fatalf("Failed to load config filter: %v", err)
	}

	// Add runtime filter that includes "TV Shows"
	if err := m.loadFilterSet("group_title", []string{"^TV Shows"}, []string{}, true); err != nil {
		t.Fatalf("Failed to load runtime filter: %v", err)
	}

	// Runtime filter should take precedence
	tests := []struct {
		value string
		want  bool
	}{
		{"Movies HD", false},     // Not matched by runtime filter
		{"TV Shows HD", true},    // Matched by runtime filter
		{"Documentaries", false}, // Not matched by runtime filter
	}

	for _, tt := range tests {
		t.Run(tt.value, func(t *testing.T) {
			got := m.Matches("group_title", tt.value)
			if got != tt.want {
				t.Errorf("Matches() = %v, want %v (runtime precedence test)", got, tt.want)
			}
		})
	}
}

func TestManager_GetFilterCount(t *testing.T) {
	m := NewManager()

	if m.GetFilterCount() != 0 {
		t.Errorf("Expected 0 filters, got %d", m.GetFilterCount())
	}

	m.loadFilterSet("group_title", []string{"^Movies"}, []string{}, false)

	if m.GetFilterCount() != 1 {
		t.Errorf("Expected 1 filter, got %d", m.GetFilterCount())
	}

	m.loadFilterSet("tvg_name", []string{"Matrix"}, []string{}, false)

	if m.GetFilterCount() != 2 {
		t.Errorf("Expected 2 filters, got %d", m.GetFilterCount())
	}
}

func BenchmarkMatches(b *testing.B) {
	m := NewManager()
	m.loadFilterSet("group_title", []string{"^Movies.*HD$"}, []string{"XXX", "Adult"}, false)

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.Matches("group_title", "Movies Action HD")
	}
}

func BenchmarkMatchesItem(b *testing.B) {
	m := NewManager()
	m.loadFilterSet("group_title", []string{"^Movies"}, []string{}, false)
	m.loadFilterSet("tvg_name", []string{".*"}, []string{"Trailer"}, false)

	item := models.ProcessedLine{
		GroupTitle: "Movies HD",
		TvgName:    "The Matrix (1999)",
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		m.MatchesItem(item)
	}
}
