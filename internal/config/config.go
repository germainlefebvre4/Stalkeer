package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config holds the application configuration
type Config struct {
	Database  DatabaseConfig  `mapstructure:"database"`
	M3U       M3UConfig       `mapstructure:"m3u"`
	Filter    FilterConfig    `mapstructure:"filter"`
	Logging   LoggingConfig   `mapstructure:"logging"`
	API       APIConfig       `mapstructure:"api"`
	TMDB      TMDBConfig      `mapstructure:"tmdb"`
	Radarr    RadarrConfig    `mapstructure:"radarr"`
	Sonarr    SonarrConfig    `mapstructure:"sonarr"`
	Downloads DownloadsConfig `mapstructure:"downloads"`
}

// DatabaseConfig holds database connection settings
type DatabaseConfig struct {
	Host     string `mapstructure:"host"`
	Port     int    `mapstructure:"port"`
	User     string `mapstructure:"user"`
	Password string `mapstructure:"password"`
	DBName   string `mapstructure:"dbname"`
	SSLMode  string `mapstructure:"sslmode"`
}

// M3UConfig holds M3U playlist settings
type M3UConfig struct {
	FilePath       string `mapstructure:"file_path"`
	UpdateInterval int    `mapstructure:"update_interval"`
}

// FilterConfig holds filter settings
type FilterConfig struct {
	GroupTitle FilterDef `mapstructure:"group_title"`
	TvgName    FilterDef `mapstructure:"tvg_name"`
}

// FilterDef represents a filter definition
type FilterDef struct {
	IncludePatterns []string `mapstructure:"include_patterns"`
	ExcludePatterns []string `mapstructure:"exclude_patterns"`
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	// Legacy field (deprecated but supported)
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`

	// New modular configuration
	App      LogLevelConfig `mapstructure:"app"`
	Database LogLevelConfig `mapstructure:"database"`
}

// LogLevelConfig represents log level configuration for a specific component
type LogLevelConfig struct {
	Level string `mapstructure:"level"` // debug, info, warn, error
}

// APIConfig holds API server settings
type APIConfig struct {
	Port int `mapstructure:"port"`
}

// TMDBConfig holds TMDB API settings
type TMDBConfig struct {
	APIKey   string `mapstructure:"api_key"`
	Language string `mapstructure:"language"`
	Enabled  bool   `mapstructure:"enabled"`
}

// RadarrConfig holds Radarr integration settings
type RadarrConfig struct {
	URL              string `mapstructure:"url"`
	APIKey           string `mapstructure:"api_key"`
	Enabled          bool   `mapstructure:"enabled"`
	SyncInterval     int    `mapstructure:"sync_interval"`
	QualityProfileID int    `mapstructure:"quality_profile_id"`
}

// SonarrConfig holds Sonarr integration settings
type SonarrConfig struct {
	URL              string `mapstructure:"url"`
	APIKey           string `mapstructure:"api_key"`
	Enabled          bool   `mapstructure:"enabled"`
	SyncInterval     int    `mapstructure:"sync_interval"`
	QualityProfileID int    `mapstructure:"quality_profile_id"`
}

// DownloadsConfig holds download settings
type DownloadsConfig struct {
	MoviesPath    string `mapstructure:"movies_path"`
	TVShowsPath   string `mapstructure:"tvshows_path"`
	TempDir       string `mapstructure:"temp_dir"`
	MaxParallel   int    `mapstructure:"max_parallel"`
	Timeout       int    `mapstructure:"timeout"`
	RetryAttempts int    `mapstructure:"retry_attempts"`
}

var cfg *Config

// Load reads configuration from file and environment variables
func Load() error {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath(".")
	viper.AddConfigPath("./config")
	viper.AddConfigPath("/etc/stalkeer")

	// Set defaults
	setDefaults()

	// Enable environment variable overrides
	viper.SetEnvPrefix("STALKEER")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Bind environment variables explicitly for nested config
	viper.BindEnv("database.host")
	viper.BindEnv("database.port")
	viper.BindEnv("database.user")
	viper.BindEnv("database.password")
	viper.BindEnv("database.dbname")
	viper.BindEnv("database.sslmode")
	viper.BindEnv("m3u.file_path")
	viper.BindEnv("m3u.update_interval")
	viper.BindEnv("logging.level")
	viper.BindEnv("logging.format")
	viper.BindEnv("logging.app.level")
	viper.BindEnv("logging.database.level")
	viper.BindEnv("api.port")
	viper.BindEnv("tmdb.api_key")
	viper.BindEnv("tmdb.language")
	viper.BindEnv("tmdb.enabled")
	viper.BindEnv("radarr.url")
	viper.BindEnv("radarr.api_key")
	viper.BindEnv("radarr.enabled")
	viper.BindEnv("sonarr.url")
	viper.BindEnv("sonarr.api_key")
	viper.BindEnv("sonarr.enabled")

	// Special handling for DATABASE_URL
	if dbURL := os.Getenv("DATABASE_URL"); dbURL != "" {
		parseDatabaseURL(dbURL)
	}

	// Read config file if it exists
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Unmarshal into Config struct
	cfg = &Config{}
	if err := viper.Unmarshal(cfg); err != nil {
		return fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate required fields
	if err := validate(); err != nil {
		return fmt.Errorf("configuration validation failed: %w", err)
	}

	return nil
}

// Get returns the current configuration
func Get() *Config {
	if cfg == nil {
		return &Config{}
	}
	return cfg
}

// Reload reloads the configuration from file
func Reload() error {
	return Load()
}

func setDefaults() {
	// Database defaults
	viper.SetDefault("database.host", "localhost")
	viper.SetDefault("database.port", 5432)
	viper.SetDefault("database.sslmode", "disable")

	// M3U defaults
	viper.SetDefault("m3u.update_interval", 3600)

	// Radarr defaults
	viper.SetDefault("radarr.enabled", false)
	viper.SetDefault("radarr.sync_interval", 3600)
	viper.SetDefault("radarr.quality_profile_id", 1)

	// Sonarr defaults
	viper.SetDefault("sonarr.enabled", false)
	viper.SetDefault("sonarr.sync_interval", 3600)
	viper.SetDefault("sonarr.quality_profile_id", 1)

	// Downloads defaults
	viper.SetDefault("downloads.movies_path", "./data/downloads/movies")
	viper.SetDefault("downloads.tvshows_path", "./data/downloads/tvshows")
	viper.SetDefault("downloads.max_parallel", 3)
	viper.SetDefault("downloads.timeout", 300)
	viper.SetDefault("downloads.retry_attempts", 3)

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")

	// TMDB defaults
	viper.SetDefault("tmdb.enabled", true)
	viper.SetDefault("tmdb.language", "en-US")

	// API defaults
	viper.SetDefault("api.port", 8080)
}

func validate() error {
	if cfg.Database.User == "" {
		return fmt.Errorf("database.user is required")
	}
	if cfg.Database.DBName == "" {
		return fmt.Errorf("database.dbname is required")
	}
	// m3u.file_path is optional - can be provided via CLI

	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}

	// Validate legacy log level if set
	if cfg.Logging.Level != "" && !validLevels[cfg.Logging.Level] {
		return fmt.Errorf("logging.level must be one of: debug, info, warn, error")
	}

	// Validate app log level if set
	if cfg.Logging.App.Level != "" && !validLevels[cfg.Logging.App.Level] {
		return fmt.Errorf("logging.app.level must be one of: debug, info, warn, error")
	}

	// Validate database log level if set
	if cfg.Logging.Database.Level != "" && !validLevels[cfg.Logging.Database.Level] {
		return fmt.Errorf("logging.database.level must be one of: debug, info, warn, error")
	}

	return nil
}

// GetAppLogLevel returns the log level for application logging
// Priority: logging.app.level → logging.level → "info"
func (c *Config) GetAppLogLevel() string {
	if c.Logging.App.Level != "" {
		return c.Logging.App.Level
	}
	if c.Logging.Level != "" {
		return c.Logging.Level
	}
	return "info"
}

// GetDatabaseLogLevel returns the log level for database logging
// Priority: logging.database.level → logging.level → "info"
func (c *Config) GetDatabaseLogLevel() string {
	if c.Logging.Database.Level != "" {
		return c.Logging.Database.Level
	}
	if c.Logging.Level != "" {
		return c.Logging.Level
	}
	return "info"
}

// IsUsingLegacyLogging returns true if using deprecated logging.level
func (c *Config) IsUsingLegacyLogging() bool {
	return c.Logging.Level != "" && c.Logging.App.Level == "" && c.Logging.Database.Level == ""
}

func parseDatabaseURL(url string) {
	// Simple DATABASE_URL parser for postgres://user:password@host:port/dbname
	// This is a basic implementation; consider using a URL parsing library for production
	if strings.HasPrefix(url, "postgres://") || strings.HasPrefix(url, "postgresql://") {
		// Remove scheme
		url = strings.TrimPrefix(url, "postgres://")
		url = strings.TrimPrefix(url, "postgresql://")

		// Split credentials and host
		parts := strings.Split(url, "@")
		if len(parts) == 2 {
			// Parse credentials
			creds := strings.Split(parts[0], ":")
			if len(creds) == 2 {
				viper.Set("database.user", creds[0])
				viper.Set("database.password", creds[1])
			}

			// Parse host, port, and database
			hostParts := strings.Split(parts[1], "/")
			if len(hostParts) == 2 {
				hostPort := strings.Split(hostParts[0], ":")
				viper.Set("database.host", hostPort[0])
				if len(hostPort) == 2 {
					viper.Set("database.port", hostPort[1])
				}
				viper.Set("database.dbname", hostParts[1])
			}
		}
	}
}
