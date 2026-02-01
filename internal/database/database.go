package database

import (
	"fmt"
	"time"

	"github.com/glefebvre/stalkeer/internal/config"
	"github.com/glefebvre/stalkeer/internal/logger"
	"github.com/glefebvre/stalkeer/internal/models"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

var db *gorm.DB

// InitializeWithRetry sets up the database connection with retry logic for container startup
func InitializeWithRetry(maxRetries int, retryDelay time.Duration) error {
	var err error
	for i := 0; i < maxRetries; i++ {
		err = Initialize()
		if err == nil {
			return nil
		}

		if i < maxRetries-1 {
			logger.AppLogger().WithFields(map[string]interface{}{
				"attempt":     i + 1,
				"max_retries": maxRetries,
				"retry_in":    retryDelay.String(),
				"error":       err.Error(),
			}).Warn("Failed to connect to database, retrying...")
			time.Sleep(retryDelay)
		}
	}
	return fmt.Errorf("failed to connect to database after %d attempts: %w", maxRetries, err)
}

// Initialize sets up the database connection and runs migrations
func Initialize() error {
	cfg := config.Get()

	dsn := fmt.Sprintf(
		"host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		cfg.Database.Host,
		cfg.Database.Port,
		cfg.Database.User,
		cfg.Database.Password,
		cfg.Database.DBName,
		cfg.Database.SSLMode,
	)

	// Create GORM logger adapter using database log level
	gormLogger := logger.NewGormAdapter(logger.DatabaseLogger(), cfg.GetDatabaseLogLevel())

	var err error
	db, err = gorm.Open(postgres.Open(dsn), &gorm.Config{
		Logger: gormLogger,
	})
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}

	// Configure connection pool
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	sqlDB.SetMaxIdleConns(10)
	sqlDB.SetMaxOpenConns(100)
	sqlDB.SetConnMaxLifetime(time.Hour)

	// Run auto-migrations
	if err := runMigrations(); err != nil {
		return fmt.Errorf("failed to run migrations: %w", err)
	}

	return nil
}

// Get returns the database instance
func Get() *gorm.DB {
	return db
}

// GetDB is an alias for Get() to maintain compatibility
func GetDB() *gorm.DB {
	return db
}

// HealthCheck verifies database connectivity
func HealthCheck() error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	if err := sqlDB.Ping(); err != nil {
		return fmt.Errorf("database ping failed: %w", err)
	}

	return nil
}

// Close closes the database connection
func Close() error {
	sqlDB, err := db.DB()
	if err != nil {
		return fmt.Errorf("failed to get database instance: %w", err)
	}

	return sqlDB.Close()
}

func runMigrations() error {
	return db.AutoMigrate(
		&models.Movie{},
		&models.TVShow{},
		&models.Channel{},
		&models.Uncategorized{},
		&models.FilterConfig{},
		&models.ProcessingLog{},
		&models.DownloadInfo{},
		&models.ProcessedLine{},
	)
}
