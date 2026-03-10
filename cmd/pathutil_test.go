package main

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestBuildSonarrDestPath_UseSeriesPath(t *testing.T) {
	t.Run("primary root folder", func(t *testing.T) {
		got, fallback := buildSonarrDestPath("/downloads/sonarr/Breaking Bad", "./data/sonarr", "Breaking Bad", 1, 1)
		if fallback {
			t.Error("expected no fallback")
		}
		want := filepath.Join("/downloads/sonarr/Breaking Bad", "Season 01", "Breaking Bad - S01E01")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("secondary root folder (sonarr-bis)", func(t *testing.T) {
		got, fallback := buildSonarrDestPath("/downloads/sonarr-bis/Malcolm in the Middle", "./data/sonarr", "Malcolm in the Middle", 1, 1)
		if fallback {
			t.Error("expected no fallback")
		}
		if !strings.HasPrefix(got, "/downloads/sonarr-bis/Malcolm in the Middle") {
			t.Errorf("expected path to start with /downloads/sonarr-bis/Malcolm in the Middle, got %q", got)
		}
		want := filepath.Join("/downloads/sonarr-bis/Malcolm in the Middle", "Season 01", "Malcolm in the Middle - S01E01")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("season and episode zero-padding", func(t *testing.T) {
		got, _ := buildSonarrDestPath("/downloads/sonarr/Show", "./data/sonarr", "Show", 3, 12)
		if !strings.HasSuffix(got, "Season 03"+string(filepath.Separator)+"Show - S03E12") {
			t.Errorf("unexpected path suffix, got %q", got)
		}
	})
}

func TestBuildSonarrDestPath_EmptyPathFallback(t *testing.T) {
	got, fallback := buildSonarrDestPath("", "./data/sonarr", "My Show", 2, 5)
	if !fallback {
		t.Error("expected fallback=true when seriesPath is empty")
	}
	want := filepath.Join("./data/sonarr", "My Show", "Season 02", "My Show - S02E05")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

func TestBuildRadarrDestPath_UseMoviePath(t *testing.T) {
	t.Run("primary root folder", func(t *testing.T) {
		got, fallback := buildRadarrDestPath("/downloads/radarr/The Matrix (1999)", "./data/radarr", "The Matrix", 1999)
		if fallback {
			t.Error("expected no fallback")
		}
		want := filepath.Join("/downloads/radarr/The Matrix (1999)", "The Matrix (1999)")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})

	t.Run("secondary root folder (4k)", func(t *testing.T) {
		got, fallback := buildRadarrDestPath("/downloads/radarr-4k/Inception (2010)", "./data/radarr", "Inception", 2010)
		if fallback {
			t.Error("expected no fallback")
		}
		want := filepath.Join("/downloads/radarr-4k/Inception (2010)", "Inception (2010)")
		if got != want {
			t.Errorf("got %q, want %q", got, want)
		}
	})
}

func TestBuildRadarrDestPath_EmptyPathFallback(t *testing.T) {
	got, fallback := buildRadarrDestPath("", "./data/radarr", "Dune", 2021)
	if !fallback {
		t.Error("expected fallback=true when moviePath is empty")
	}
	want := filepath.Join("./data/radarr", "Dune (2021)", "Dune (2021)")
	if got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}
