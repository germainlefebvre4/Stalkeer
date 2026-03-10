package models

import "time"

// ProcessingLog represents a log entry for processing actions
type ProcessingLog struct {
	ID           uint       `gorm:"primaryKey" json:"id"`
	Action       string     `gorm:"type:varchar(100);not null" json:"action"`
	ItemCount    int        `gorm:"not null;default:0" json:"item_count"`
	Status       string     `gorm:"type:varchar(50);not null" json:"status"` // "success", "failed", "in_progress"
	StartedAt    time.Time  `gorm:"not null" json:"started_at"`
	CompletedAt  *time.Time `json:"completed_at,omitempty"`
	ErrorMessage *string    `gorm:"type:text" json:"error_message,omitempty"`
	CreatedAt    time.Time  `gorm:"not null" json:"created_at"`
	UpdatedAt    time.Time  `gorm:"not null" json:"updated_at"`
}

// TableName specifies the table name for ProcessingLog
func (ProcessingLog) TableName() string {
	return "processing_logs"
}
