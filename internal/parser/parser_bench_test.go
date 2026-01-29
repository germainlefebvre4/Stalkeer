package parser

import (
	"os"
	"testing"
)

func BenchmarkParse100(b *testing.B) {
	testFile := "/home/glefebvre/Documents/Dev/Perso/TorrentTracker/stalkeer/m3u_playlist/test_100_entries.m3u"
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		b.Skip("test file not found")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser := NewParser(testFile)
		_, err := parser.Parse()
		if err != nil {
			b.Fatalf("Parse failed: %v", err)
		}
	}
}

func BenchmarkParse1000(b *testing.B) {
	testFile := "/home/glefebvre/Documents/Dev/Perso/TorrentTracker/stalkeer/m3u_playlist/test_1000_entries.m3u"
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		b.Skip("test file not found")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser := NewParser(testFile)
		_, err := parser.Parse()
		if err != nil {
			b.Fatalf("Parse failed: %v", err)
		}
	}
}

func BenchmarkParse10000(b *testing.B) {
	testFile := "/home/glefebvre/Documents/Dev/Perso/TorrentTracker/stalkeer/m3u_playlist/test_10000_entries.m3u"
	if _, err := os.Stat(testFile); os.IsNotExist(err) {
		b.Skip("test file not found")
	}

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		parser := NewParser(testFile)
		_, err := parser.Parse()
		if err != nil {
			b.Fatalf("Parse failed: %v", err)
		}
	}
}

func BenchmarkCalculateHash(b *testing.B) {
	parser := NewParser("")
	title := "Test Movie Title"
	url := "http://example.com/movie.mkv"

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = parser.calculateHash(title, url)
	}
}

func BenchmarkParseExtinf(b *testing.B) {
	parser := NewParser("")
	line := `#EXTINF:-1 tvg-id="movie1" tvg-name="Test Movie" tvg-logo="http://example.com/logo.jpg" group-title="Movies",Test Movie`

	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_ = parser.parseExtinf(line, 1)
	}
}
