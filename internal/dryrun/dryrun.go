package dryrun

import (
	"fmt"
	"time"

	"github.com/glefebvre/stalkeer/internal/classifier"
	"github.com/glefebvre/stalkeer/internal/filter"
	"github.com/glefebvre/stalkeer/internal/models"
	"github.com/glefebvre/stalkeer/internal/parser"
)

// Issue represents a detected issue in the dry-run
type Issue struct {
	TvgName    string   `json:"tvg_name"`
	GroupTitle string   `json:"group_title"`
	Issues     []string `json:"issues"`
	Severity   string   `json:"severity"` // "info", "warning", "error"
}

// Result represents the result of a dry-run analysis
type Result struct {
	TotalProcessed  int     `json:"total_processed"`
	Timestamp       string  `json:"timestamp"`
	Unclassified    []Issue `json:"unclassified"`
	MissingMetadata []Issue `json:"missing_metadata"`
	FilteredOut     []Issue `json:"filtered_out"`
	Duplicates      []Issue `json:"duplicates"`
	Summary         Summary `json:"summary"`
}

// Summary provides aggregate statistics
type Summary struct {
	TotalIssues   int            `json:"total_issues"`
	ByCategory    map[string]int `json:"by_category"`
	BySeverity    map[string]int `json:"by_severity"`
	ByContentType map[string]int `json:"by_content_type"`
}

// Analyzer performs dry-run analysis
type Analyzer struct {
	classifier    *classifier.Classifier
	filterManager *filter.Manager
	limit         int
	seenHashes    map[string]bool
}

// NewAnalyzer creates a new dry-run analyzer
func NewAnalyzer(limit int) *Analyzer {
	return &Analyzer{
		classifier:    classifier.New(),
		filterManager: filter.NewManager(),
		limit:         limit,
		seenHashes:    make(map[string]bool),
	}
}

// Analyze performs dry-run analysis on an M3U file
func (a *Analyzer) Analyze(filePath string) (*Result, error) {
	// Load filters from config
	if err := a.filterManager.LoadFromConfig(); err != nil {
		return nil, fmt.Errorf("failed to load filters: %w", err)
	}

	// Parse M3U file
	p := parser.NewParser(filePath)
	lines, err := p.Parse()
	if err != nil {
		return nil, fmt.Errorf("failed to parse M3U file: %w", err)
	}

	// Limit number of items to analyze
	if a.limit > 0 && len(lines) > a.limit {
		lines = lines[:a.limit]
	}

	result := &Result{
		TotalProcessed:  len(lines),
		Timestamp:       time.Now().Format(time.RFC3339),
		Unclassified:    make([]Issue, 0),
		MissingMetadata: make([]Issue, 0),
		FilteredOut:     make([]Issue, 0),
		Duplicates:      make([]Issue, 0),
		Summary: Summary{
			ByCategory:    make(map[string]int),
			BySeverity:    make(map[string]int),
			ByContentType: make(map[string]int),
		},
	}

	// Analyze each item
	for _, line := range lines {
		a.analyzeItem(line, result)
	}

	// Calculate summary
	result.Summary.TotalIssues = len(result.Unclassified) + len(result.MissingMetadata) + len(result.FilteredOut) + len(result.Duplicates)
	result.Summary.ByCategory["unclassified"] = len(result.Unclassified)
	result.Summary.ByCategory["missing_metadata"] = len(result.MissingMetadata)
	result.Summary.ByCategory["filtered_out"] = len(result.FilteredOut)
	result.Summary.ByCategory["duplicates"] = len(result.Duplicates)

	return result, nil
}

func (a *Analyzer) analyzeItem(line models.ProcessedLine, result *Result) {
	issues := make([]string, 0)
	severity := "info"

	// Check for duplicates
	if a.seenHashes[line.LineHash] {
		result.Duplicates = append(result.Duplicates, Issue{
			TvgName:    line.TvgName,
			GroupTitle: line.GroupTitle,
			Issues:     []string{"duplicate_entry"},
			Severity:   "warning",
		})
		result.Summary.BySeverity["warning"]++
		return
	}
	a.seenHashes[line.LineHash] = true

	// Check if item passes filters
	if !a.filterManager.MatchesItem(line) {
		result.FilteredOut = append(result.FilteredOut, Issue{
			TvgName:    line.TvgName,
			GroupTitle: line.GroupTitle,
			Issues:     []string{"filtered_out_by_rules"},
			Severity:   "info",
		})
		result.Summary.BySeverity["info"]++
		return
	}

	// Classify content
	classification := a.classifier.Classify(line.TvgName, line.GroupTitle)

	// Track content type
	result.Summary.ByContentType[string(classification.ContentType)]++

	// Check for low confidence classification
	if classification.Confidence < 50 {
		issues = append(issues, "low_confidence_classification")
		severity = "warning"
	}

	// Check for unclassified content
	if classification.ContentType == classifier.ContentTypeUncategorized {
		issues = append(issues, "content_type_uncategorized")
		severity = "warning"
	}

	// Check for missing metadata
	if classification.ContentType == classifier.ContentTypeSeries {
		if classification.Season == nil || classification.Episode == nil {
			issues = append(issues, "missing_season_episode")
			severity = "warning"
		}
	}

	if classification.Resolution == nil {
		issues = append(issues, "missing_resolution")
		if severity == "info" {
			severity = "info"
		}
	}

	// Add to appropriate category
	if len(issues) > 0 {
		issue := Issue{
			TvgName:    line.TvgName,
			GroupTitle: line.GroupTitle,
			Issues:     issues,
			Severity:   severity,
		}

		if classification.ContentType == classifier.ContentTypeUncategorized || classification.Confidence < 50 {
			result.Unclassified = append(result.Unclassified, issue)
		} else {
			result.MissingMetadata = append(result.MissingMetadata, issue)
		}

		result.Summary.BySeverity[severity]++
	}
}

// PrintSummary prints a human-readable summary of the dry-run results
func PrintSummary(result *Result) {
	fmt.Println("\n=== Dry-Run Analysis Summary ===")
	fmt.Printf("Timestamp: %s\n", result.Timestamp)
	fmt.Printf("Total Processed: %d items\n", result.TotalProcessed)
	fmt.Printf("Total Issues: %d\n\n", result.Summary.TotalIssues)

	fmt.Println("By Category:")
	for category, count := range result.Summary.ByCategory {
		if count > 0 {
			fmt.Printf("  - %s: %d\n", category, count)
		}
	}

	fmt.Println("\nBy Severity:")
	for severity, count := range result.Summary.BySeverity {
		if count > 0 {
			fmt.Printf("  - %s: %d\n", severity, count)
		}
	}

	fmt.Println("\nBy Content Type:")
	for contentType, count := range result.Summary.ByContentType {
		if count > 0 {
			fmt.Printf("  - %s: %d\n", contentType, count)
		}
	}

	// Print sample issues
	if len(result.Unclassified) > 0 {
		fmt.Println("\n=== Sample Unclassified Items (first 5) ===")
		for i, issue := range result.Unclassified {
			if i >= 5 {
				break
			}
			fmt.Printf("\n%d. %s\n", i+1, issue.TvgName)
			fmt.Printf("   Group: %s\n", issue.GroupTitle)
			fmt.Printf("   Issues: %v\n", issue.Issues)
			fmt.Printf("   Severity: %s\n", issue.Severity)
		}
	}

	if len(result.MissingMetadata) > 0 {
		fmt.Println("\n=== Sample Missing Metadata Items (first 5) ===")
		for i, issue := range result.MissingMetadata {
			if i >= 5 {
				break
			}
			fmt.Printf("\n%d. %s\n", i+1, issue.TvgName)
			fmt.Printf("   Group: %s\n", issue.GroupTitle)
			fmt.Printf("   Issues: %v\n", issue.Issues)
			fmt.Printf("   Severity: %s\n", issue.Severity)
		}
	}

	if len(result.FilteredOut) > 0 {
		fmt.Printf("\n=== Filtered Out: %d items ===\n", len(result.FilteredOut))
	}

	if len(result.Duplicates) > 0 {
		fmt.Printf("\n=== Duplicates: %d items ===\n", len(result.Duplicates))
	}
}
