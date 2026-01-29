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
