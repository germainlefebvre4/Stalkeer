package config

import (
	"fmt"
	"os"
	"strings"

	"github.com/spf13/viper"
)

// Config holds the application configuration
type Config struct {
	Database DatabaseConfig `mapstructure:"database"`
	M3U      M3UConfig      `mapstructure:"m3u"`
	Filters  []FilterDef    `mapstructure:"filters"`
	Logging  LoggingConfig  `mapstructure:"logging"`
	API      APIConfig      `mapstructure:"api"`
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

// FilterDef represents a filter definition
type FilterDef struct {
	Name            string   `mapstructure:"name"`
	IncludePatterns []string `mapstructure:"include_patterns"`
	ExcludePatterns []string `mapstructure:"exclude_patterns"`
}

// LoggingConfig holds logging settings
type LoggingConfig struct {
	Level  string `mapstructure:"level"`
	Format string `mapstructure:"format"`
}

// APIConfig holds API server settings
type APIConfig struct {
	Port int `mapstructure:"port"`
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
	viper.BindEnv("api.port")

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

	// Logging defaults
	viper.SetDefault("logging.level", "info")
	viper.SetDefault("logging.format", "json")

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
	if cfg.M3U.FilePath == "" {
		return fmt.Errorf("m3u.file_path is required")
	}

	validLevels := map[string]bool{"debug": true, "info": true, "warn": true, "error": true}
	if !validLevels[cfg.Logging.Level] {
		return fmt.Errorf("logging.level must be one of: debug, info, warn, error")
	}

	return nil
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
