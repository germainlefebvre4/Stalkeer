package models

import "time"

// FilterConfig represents runtime filters stored in the database
type FilterConfig struct {
	ID              uint      `gorm:"primaryKey" json:"id"`
	Name            string    `gorm:"type:varchar(255);not null;uniqueIndex" json:"name"`
	Attribute       string    `gorm:"type:varchar(50);not null" json:"attribute"`  // "group_title" or "tvg_name"
	IncludePatterns *string   `gorm:"type:text" json:"include_patterns,omitempty"` // JSON array
	ExcludePatterns *string   `gorm:"type:text" json:"exclude_patterns,omitempty"` // JSON array
	IsRuntime       bool      `gorm:"not null;default:true;index" json:"is_runtime"`
	CreatedAt       time.Time `gorm:"not null" json:"created_at"`
	UpdatedAt       time.Time `gorm:"not null" json:"updated_at"`
}

// TableName specifies the table name for FilterConfig
func (FilterConfig) TableName() string {
	return "filter_configs"
}
