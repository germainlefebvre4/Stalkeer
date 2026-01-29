package filter

import (
	"encoding/json"
	"fmt"
	"regexp"

	"github.com/glefebvre/stalkeer/internal/config"
	"github.com/glefebvre/stalkeer/internal/database"
	"github.com/glefebvre/stalkeer/internal/models"
)

// Filter represents a compiled filter
type Filter struct {
	Name            string
	Attribute       string // "group_title" or "tvg_name"
	IncludePatterns []*regexp.Regexp
	ExcludePatterns []*regexp.Regexp
	IsRuntime       bool
}

// Manager handles filter operations
type Manager struct {
	filters []Filter
}

// NewManager creates a new filter manager
func NewManager() *Manager {
	return &Manager{
		filters: make([]Filter, 0),
	}
}

// LoadFromConfig loads file-based filters from configuration
func (m *Manager) LoadFromConfig() error {
	cfg := config.Get()

	// Load group-title filters
	if err := m.loadFilterSet("group_title", cfg.Filter.GroupTitle.IncludePatterns, cfg.Filter.GroupTitle.ExcludePatterns, false); err != nil {
		return fmt.Errorf("failed to load group-title filters: %w", err)
	}

	// Load tvg-name filters
	if err := m.loadFilterSet("tvg_name", cfg.Filter.TvgName.IncludePatterns, cfg.Filter.TvgName.ExcludePatterns, false); err != nil {
		return fmt.Errorf("failed to load tvg-name filters: %w", err)
	}

	return nil
}

// LoadFromDatabase loads runtime filters from database
func (m *Manager) LoadFromDatabase() error {
	db := database.Get()
	if db == nil {
		return fmt.Errorf("database not initialized")
	}

	var dbFilters []models.FilterConfig
	if err := db.Where("is_runtime = ?", true).Find(&dbFilters).Error; err != nil {
		return fmt.Errorf("failed to load runtime filters from database: %w", err)
	}

	for _, dbFilter := range dbFilters {
		var includePatterns []string
		var excludePatterns []string

		if dbFilter.IncludePatterns != nil {
			if err := json.Unmarshal([]byte(*dbFilter.IncludePatterns), &includePatterns); err != nil {
				return fmt.Errorf("failed to unmarshal include patterns for filter '%s': %w", dbFilter.Name, err)
			}
		}

		if dbFilter.ExcludePatterns != nil {
			if err := json.Unmarshal([]byte(*dbFilter.ExcludePatterns), &excludePatterns); err != nil {
				return fmt.Errorf("failed to unmarshal exclude patterns for filter '%s': %w", dbFilter.Name, err)
			}
		}

		if err := m.loadFilterSet(dbFilter.Attribute, includePatterns, excludePatterns, true); err != nil {
			return fmt.Errorf("failed to load runtime filter '%s': %w", dbFilter.Name, err)
		}
	}

	return nil
}

// LoadAll loads both config-based and database-based filters
func (m *Manager) LoadAll() error {
	// Load file-based filters first
	if err := m.LoadFromConfig(); err != nil {
		return err
	}

	// Load runtime filters (these override file-based filters)
	if err := m.LoadFromDatabase(); err != nil {
		return err
	}

	return nil
}

// Matches checks if an item matches the filters
func (m *Manager) Matches(attribute, value string) bool {
	// Find applicable filters
	var applicableFilters []Filter
	for _, filter := range m.filters {
		if filter.Attribute == attribute {
			applicableFilters = append(applicableFilters, filter)
		}
	}

	if len(applicableFilters) == 0 {
		// No filters for this attribute, allow all
		return true
	}

	// Runtime filters take precedence
	var runtimeFilters []Filter
	var configFilters []Filter
	for _, filter := range applicableFilters {
		if filter.IsRuntime {
			runtimeFilters = append(runtimeFilters, filter)
		} else {
			configFilters = append(configFilters, filter)
		}
	}

	// Use runtime filters if available, otherwise use config filters
	filtersToApply := configFilters
	if len(runtimeFilters) > 0 {
		filtersToApply = runtimeFilters
	}

	// Apply filters
	for _, filter := range filtersToApply {
		// Check exclude patterns first
		for _, excludePattern := range filter.ExcludePatterns {
			if excludePattern.MatchString(value) {
				return false // Excluded
			}
		}

		// If there are include patterns, at least one must match
		if len(filter.IncludePatterns) > 0 {
			matched := false
			for _, includePattern := range filter.IncludePatterns {
				if includePattern.MatchString(value) {
					matched = true
					break
				}
			}
			if !matched {
				return false // Didn't match any include pattern
			}
		}
	}

	return true
}

// ShouldProcess checks if an entry should be processed based on group-title and tvg-name
func (m *Manager) ShouldProcess(groupTitle, tvgName string) bool {
	// Check group_title filter
	if !m.Matches("group_title", groupTitle) {
		return false
	}

	// Check tvg_name filter
	if !m.Matches("tvg_name", tvgName) {
		return false
	}

	return true
}

// MatchesItem checks if a processed line matches all applicable filters
func (m *Manager) MatchesItem(item models.ProcessedLine) bool {
	// Check group_title filter
	if !m.Matches("group_title", item.GroupTitle) {
		return false
	}

	// Check tvg_name filter
	if !m.Matches("tvg_name", item.TvgName) {
		return false
	}

	return true
}

// loadFilterSet loads and compiles a set of filter patterns
func (m *Manager) loadFilterSet(attribute string, includePatterns, excludePatterns []string, isRuntime bool) error {
	filter := Filter{
		Name:            fmt.Sprintf("%s_filter", attribute),
		Attribute:       attribute,
		IncludePatterns: make([]*regexp.Regexp, 0),
		ExcludePatterns: make([]*regexp.Regexp, 0),
		IsRuntime:       isRuntime,
	}

	// Compile include patterns
	for _, pattern := range includePatterns {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("failed to compile include pattern '%s': %w", pattern, err)
		}
		filter.IncludePatterns = append(filter.IncludePatterns, compiled)
	}

	// Compile exclude patterns
	for _, pattern := range excludePatterns {
		compiled, err := regexp.Compile(pattern)
		if err != nil {
			return fmt.Errorf("failed to compile exclude pattern '%s': %w", pattern, err)
		}
		filter.ExcludePatterns = append(filter.ExcludePatterns, compiled)
	}

	// Only add filter if it has patterns
	if len(filter.IncludePatterns) > 0 || len(filter.ExcludePatterns) > 0 {
		m.filters = append(m.filters, filter)
	}

	return nil
}

// ValidatePattern validates a regex pattern
func ValidatePattern(pattern string) error {
	_, err := regexp.Compile(pattern)
	if err != nil {
		return fmt.Errorf("invalid regex pattern: %w", err)
	}
	return nil
}

// GetFilterCount returns the number of loaded filters
func (m *Manager) GetFilterCount() int {
	return len(m.filters)
}
