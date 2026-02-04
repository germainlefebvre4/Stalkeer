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
	FilePath       string            `mapstructure:"file_path"`
	UpdateInterval int               `mapstructure:"update_interval"`
	Download       M3UDownloadConfig `mapstructure:"download"`
}

// M3UDownloadConfig holds M3U download settings
type M3UDownloadConfig struct {
	Enabled         bool   `mapstructure:"enabled"`
	URL             string `mapstructure:"url"`
	ArchiveDir      string `mapstructure:"archive_dir"`
	RetentionCount  int    `mapstructure:"retention_count"`
	MaxFileSizeMB   int64  `mapstructure:"max_file_size_mb"`
	TimeoutSeconds  int    `mapstructure:"timeout_seconds"`
	RetryAttempts   int    `mapstructure:"retry_attempts"`
	AuthUsername    string `mapstructure:"auth_username"`
	AuthPassword    string `mapstructure:"auth_password"`
	ScheduleEnabled bool   `mapstructure:"schedule_enabled"`
	IntervalHours   int    `mapstructure:"interval_hours"`
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
	MoviesPath              string `mapstructure:"movies_path"`
	TVShowsPath             string `mapstructure:"tvshows_path"`
	TempDir                 string `mapstructure:"temp_dir"`
	MaxParallel             int    `mapstructure:"max_parallel"`
	Timeout                 int    `mapstructure:"timeout"`
	RetryAttempts           int    `mapstructure:"retry_attempts"`
	ResumeEnabled           bool   `mapstructure:"resume_enabled"`
	ProgressIntervalMB      int64  `mapstructure:"progress_interval_mb"`
	ProgressIntervalSeconds int    `mapstructure:"progress_interval_seconds"`
	LockTimeoutMinutes      int    `mapstructure:"lock_timeout_minutes"`
	MaxRetryAttempts        int    `mapstructure:"max_retry_attempts"`
}

var cfg *Config

// bindEnvWithAlternatives binds a viper key to environment variables with alternative names
// This allows supporting both STALKEER_DATABASE_HOST and DB_HOST for the same config key
func bindEnvWithAlternatives(key string, alternatives ...string) {
	viper.BindEnv(key)
	for _, alt := range alternatives {
		if value := os.Getenv(alt); value != "" {
			viper.Set(key, value)
			break
		}
	}
}

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
	// Support both STALKEER_ prefix and Docker-style env vars (DB_HOST, DB_PORT, etc.)
	bindEnvWithAlternatives("database.host", "DB_HOST")
	bindEnvWithAlternatives("database.port", "DB_PORT")
	bindEnvWithAlternatives("database.user", "DB_USER")
	bindEnvWithAlternatives("database.password", "DB_PASSWORD")
	bindEnvWithAlternatives("database.dbname", "DB_NAME")
	bindEnvWithAlternatives("database.sslmode", "DB_SSLMODE")

	bindEnvWithAlternatives("m3u.file_path", "M3U_FILE_PATH")
	viper.BindEnv("m3u.update_interval")
	viper.BindEnv("m3u.download.enabled")
	bindEnvWithAlternatives("m3u.download.url", "M3U_DOWNLOAD_URL")
	viper.BindEnv("m3u.download.archive_dir")
	viper.BindEnv("m3u.download.retention_count")
	viper.BindEnv("m3u.download.max_file_size_mb")
	viper.BindEnv("m3u.download.timeout_seconds")
	viper.BindEnv("m3u.download.retry_attempts")
	viper.BindEnv("m3u.download.auth_username")
	viper.BindEnv("m3u.download.auth_password")
	viper.BindEnv("m3u.download.schedule_enabled")
	viper.BindEnv("m3u.download.interval_hours")

	bindEnvWithAlternatives("logging.level", "LOG_LEVEL")
	viper.BindEnv("logging.format")
	viper.BindEnv("logging.app.level")
	viper.BindEnv("logging.database.level")

	bindEnvWithAlternatives("api.port", "API_PORT")

	bindEnvWithAlternatives("tmdb.api_key", "TMDB_API_KEY")
	viper.BindEnv("tmdb.language")
	viper.BindEnv("tmdb.enabled")

	bindEnvWithAlternatives("radarr.url", "RADARR_URL")
	bindEnvWithAlternatives("radarr.api_key", "RADARR_API_KEY")
	viper.BindEnv("radarr.enabled")
	viper.BindEnv("radarr.sync_interval")
	viper.BindEnv("radarr.quality_profile_id")

	bindEnvWithAlternatives("sonarr.url", "SONARR_URL")
	bindEnvWithAlternatives("sonarr.api_key", "SONARR_API_KEY")
	viper.BindEnv("sonarr.enabled")
	viper.BindEnv("sonarr.sync_interval")
	viper.BindEnv("sonarr.quality_profile_id")

	bindEnvWithAlternatives("downloads.movies_path", "MOVIES_PATH")
	bindEnvWithAlternatives("downloads.tvshows_path", "TVSHOWS_PATH")
	bindEnvWithAlternatives("downloads.temp_dir", "TEMP_DIR")
	bindEnvWithAlternatives("downloads.max_parallel", "MAX_PARALLEL")
	bindEnvWithAlternatives("downloads.timeout", "DOWNLOAD_TIMEOUT")
	bindEnvWithAlternatives("downloads.retry_attempts", "RETRY_ATTEMPTS")

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
	viper.SetDefault("m3u.download.enabled", false)
	viper.SetDefault("m3u.download.archive_dir", "./m3u_playlist")
	viper.SetDefault("m3u.download.retention_count", 5)
	viper.SetDefault("m3u.download.max_file_size_mb", 500)
	viper.SetDefault("m3u.download.timeout_seconds", 300)
	viper.SetDefault("m3u.download.retry_attempts", 3)
	viper.SetDefault("m3u.download.schedule_enabled", false)
	viper.SetDefault("m3u.download.interval_hours", 24)

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
	viper.SetDefault("downloads.max_parallel", 0)
	viper.SetDefault("downloads.timeout", 300)
	viper.SetDefault("downloads.retry_attempts", 3)
	viper.SetDefault("downloads.resume_enabled", true)
	viper.SetDefault("downloads.progress_interval_mb", 10)
	viper.SetDefault("downloads.progress_interval_seconds", 30)
	viper.SetDefault("downloads.lock_timeout_minutes", 5)
	viper.SetDefault("downloads.max_retry_attempts", 5)

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
	validFormats := map[string]bool{"json": true, "text": true}

	// Validate logging format if set
	if cfg.Logging.Format != "" && !validFormats[cfg.Logging.Format] {
		return fmt.Errorf("logging.format must be one of: json, text")
	}

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
