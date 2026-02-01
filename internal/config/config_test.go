package config

import (
	"os"
	"strings"
	"testing"
)

func TestLoad_WithDefaults(t *testing.T) {
	// Set required environment variables
	os.Setenv("STALKEER_DATABASE_USER", "testuser")
	os.Setenv("STALKEER_DATABASE_DBNAME", "testdb")
	os.Setenv("STALKEER_M3U_FILE_PATH", "/tmp/test.m3u")
	defer func() {
		os.Unsetenv("STALKEER_DATABASE_USER")
		os.Unsetenv("STALKEER_DATABASE_DBNAME")
		os.Unsetenv("STALKEER_M3U_FILE_PATH")
	}()

	// Reset cfg to nil to force reload
	cfg = nil

	err := Load()
	if err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	config := Get()
	if config.Database.Host != "localhost" {
		t.Errorf("expected default host 'localhost', got %s", config.Database.Host)
	}
	if config.Database.Port != 5432 {
		t.Errorf("expected default port 5432, got %d", config.Database.Port)
	}
	if config.Logging.Level != "info" {
		t.Errorf("expected default log level 'info', got %s", config.Logging.Level)
	}
	if config.API.Port != 8080 {
		t.Errorf("expected default API port 8080, got %d", config.API.Port)
	}
}

func TestValidate_InvalidLogLevel(t *testing.T) {
	os.Setenv("STALKEER_DATABASE_USER", "testuser")
	os.Setenv("STALKEER_DATABASE_DBNAME", "testdb")
	os.Setenv("STALKEER_M3U_FILE_PATH", "/tmp/test.m3u")
	os.Setenv("STALKEER_LOGGING_LEVEL", "invalid")
	defer func() {
		os.Unsetenv("STALKEER_DATABASE_USER")
		os.Unsetenv("STALKEER_DATABASE_DBNAME")
		os.Unsetenv("STALKEER_M3U_FILE_PATH")
		os.Unsetenv("STALKEER_LOGGING_LEVEL")
	}()

	cfg = nil
	err := Load()
	if err == nil {
		t.Fatalf("expected error for invalid log level, got nil")
	}
	if !strings.Contains(err.Error(), "logging.level must be one of") {
		t.Errorf("expected error about log level, got: %s", err.Error())
	}
}

func TestGetAppLogLevel_ModularConfig(t *testing.T) {
	cfg := &Config{
		Logging: LoggingConfig{
			App: LogLevelConfig{Level: "debug"},
		},
	}

	level := cfg.GetAppLogLevel()
	if level != "debug" {
		t.Errorf("expected app log level 'debug', got %s", level)
	}
}

func TestGetAppLogLevel_LegacyFallback(t *testing.T) {
	cfg := &Config{
		Logging: LoggingConfig{
			Level: "warn",
		},
	}

	level := cfg.GetAppLogLevel()
	if level != "warn" {
		t.Errorf("expected app log level 'warn' from legacy config, got %s", level)
	}
}

func TestGetAppLogLevel_DefaultFallback(t *testing.T) {
	cfg := &Config{
		Logging: LoggingConfig{},
	}

	level := cfg.GetAppLogLevel()
	if level != "info" {
		t.Errorf("expected default app log level 'info', got %s", level)
	}
}

func TestGetAppLogLevel_Priority(t *testing.T) {
	cfg := &Config{
		Logging: LoggingConfig{
			Level: "warn",
			App:   LogLevelConfig{Level: "debug"},
		},
	}

	level := cfg.GetAppLogLevel()
	if level != "debug" {
		t.Errorf("expected app.level to take priority over legacy level, got %s", level)
	}
}

func TestGetDatabaseLogLevel_ModularConfig(t *testing.T) {
	cfg := &Config{
		Logging: LoggingConfig{
			Database: LogLevelConfig{Level: "error"},
		},
	}

	level := cfg.GetDatabaseLogLevel()
	if level != "error" {
		t.Errorf("expected database log level 'error', got %s", level)
	}
}

func TestGetDatabaseLogLevel_LegacyFallback(t *testing.T) {
	cfg := &Config{
		Logging: LoggingConfig{
			Level: "debug",
		},
	}

	level := cfg.GetDatabaseLogLevel()
	if level != "debug" {
		t.Errorf("expected database log level 'debug' from legacy config, got %s", level)
	}
}

func TestIsUsingLegacyLogging_True(t *testing.T) {
	cfg := &Config{
		Logging: LoggingConfig{
			Level: "info",
		},
	}

	if !cfg.IsUsingLegacyLogging() {
		t.Error("expected IsUsingLegacyLogging to return true")
	}
}

func TestIsUsingLegacyLogging_False_WithModularConfig(t *testing.T) {
	cfg := &Config{
		Logging: LoggingConfig{
			Level: "info",
			App:   LogLevelConfig{Level: "debug"},
		},
	}

	if cfg.IsUsingLegacyLogging() {
		t.Error("expected IsUsingLegacyLogging to return false when modular config is set")
	}
}

func TestIsUsingLegacyLogging_False_NoLegacy(t *testing.T) {
	cfg := &Config{
		Logging: LoggingConfig{
			App:      LogLevelConfig{Level: "debug"},
			Database: LogLevelConfig{Level: "warn"},
		},
	}

	if cfg.IsUsingLegacyLogging() {
		t.Error("expected IsUsingLegacyLogging to return false when only modular config is set")
	}
}

func TestValidate_ModularLogLevels(t *testing.T) {
	tests := []struct {
		name        string
		appLevel    string
		dbLevel     string
		expectError bool
	}{
		{"valid debug levels", "debug", "debug", false},
		{"valid info levels", "info", "info", false},
		{"valid warn levels", "warn", "warn", false},
		{"valid error levels", "error", "error", false},
		{"invalid app level", "invalid", "info", true},
		{"invalid db level", "info", "invalid", true},
		{"both invalid", "bad", "wrong", true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cfg = &Config{
				Database: DatabaseConfig{
					User:   "testuser",
					DBName: "testdb",
				},
				Logging: LoggingConfig{
					App:      LogLevelConfig{Level: tt.appLevel},
					Database: LogLevelConfig{Level: tt.dbLevel},
				},
			}

			err := validate()
			if tt.expectError && err == nil {
				t.Error("expected validation error, got nil")
			}
			if !tt.expectError && err != nil {
				t.Errorf("unexpected validation error: %v", err)
			}
		})
	}
}
