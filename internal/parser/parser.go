package parser

import (
	"bufio"
	"fmt"
	"os"
	"strings"

	"github.com/glefebvre/stalkeer/internal/models"
)

// Parser handles M3U playlist parsing
type Parser struct {
	filePath string
}

// NewParser creates a new parser instance
func NewParser(filePath string) *Parser {
	return &Parser{
		filePath: filePath,
	}
}

// Parse reads and parses an M3U playlist file
func (p *Parser) Parse() ([]models.ProcessedLine, error) {
	file, err := os.Open(p.filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to open playlist file: %w", err)
	}
	defer file.Close()

	var lines []models.ProcessedLine
	scanner := bufio.NewScanner(file)

	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments that are not EXTINF
		if line == "" || (strings.HasPrefix(line, "#") && !strings.HasPrefix(line, "#EXTINF")) {
			continue
		}

		// Parse EXTINF line
		if strings.HasPrefix(line, "#EXTINF") {
			// TODO: Implement EXTINF parsing logic
			// Extract tvg-name, group-title, tvg-logo, etc.
			continue
		}

		// Parse stream URL
		if !strings.HasPrefix(line, "#") {
			// TODO: Create ProcessedLine from parsed data
			// lines = append(lines, line)
			continue
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading playlist file: %w", err)
	}

	return lines, nil
}
