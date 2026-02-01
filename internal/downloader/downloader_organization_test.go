package downloader

import (
	"testing"
)

func TestDetectFileExtension(t *testing.T) {
	tests := []struct {
		name        string
		url         string
		contentType string
		expected    string
	}{
		{
			name:        "URL with .mkv extension",
			url:         "http://example.com/video.mkv",
			contentType: "",
			expected:    ".mkv",
		},
		{
			name:        "URL with .mp4 extension",
			url:         "http://example.com/movie.mp4",
			contentType: "",
			expected:    ".mp4",
		},
		{
			name:        "URL with query parameters",
			url:         "http://example.com/video.avi?token=abc123",
			contentType: "",
			expected:    ".avi",
		},
		{
			name:        "No extension in URL, use Content-Type",
			url:         "http://example.com/stream",
			contentType: "video/x-matroska",
			expected:    ".mkv",
		},
		{
			name:        "No extension in URL, MP4 content type",
			url:         "http://example.com/media/12345",
			contentType: "video/mp4",
			expected:    ".mp4",
		},
		{
			name:        "Content-Type with charset",
			url:         "http://example.com/video",
			contentType: "video/webm; charset=utf-8",
			expected:    ".webm",
		},
		{
			name:        "Unknown content type, default to mkv",
			url:         "http://example.com/stream",
			contentType: "application/octet-stream",
			expected:    ".mkv",
		},
		{
			name:        "No URL extension and no content type",
			url:         "http://example.com/file",
			contentType: "",
			expected:    ".mkv",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			result := detectFileExtension(tt.url, tt.contentType)
			if result != tt.expected {
				t.Errorf("detectFileExtension(%q, %q) = %q, want %q",
					tt.url, tt.contentType, result, tt.expected)
			}
		})
	}
}
