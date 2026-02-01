package logger

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
	gormlogger "gorm.io/gorm/logger"
)

// GormAdapter adapts our Logger to implement gorm's logger.Interface
type GormAdapter struct {
	logger                    *Logger
	logLevel                  gormlogger.LogLevel
	slowThreshold             time.Duration
	ignoreRecordNotFoundError bool
}

// NewGormAdapter creates a new GORM logger adapter
func NewGormAdapter(logger *Logger, level string) *GormAdapter {
	return &GormAdapter{
		logger:                    logger,
		logLevel:                  mapToGormLevel(level),
		slowThreshold:             200 * time.Millisecond,
		ignoreRecordNotFoundError: true,
	}
}

// LogMode sets the log level
func (g *GormAdapter) LogMode(level gormlogger.LogLevel) gormlogger.Interface {
	newAdapter := *g
	newAdapter.logLevel = level
	return &newAdapter
}

// Info logs info level messages
func (g *GormAdapter) Info(ctx context.Context, msg string, data ...interface{}) {
	if g.logLevel >= gormlogger.Info {
		g.logger.Info(fmt.Sprintf(msg, data...))
	}
}

// Warn logs warn level messages
func (g *GormAdapter) Warn(ctx context.Context, msg string, data ...interface{}) {
	if g.logLevel >= gormlogger.Warn {
		g.logger.Warn(fmt.Sprintf(msg, data...))
	}
}

// Error logs error level messages
func (g *GormAdapter) Error(ctx context.Context, msg string, data ...interface{}) {
	if g.logLevel >= gormlogger.Error {
		g.logger.Error(fmt.Sprintf(msg, data...), nil)
	}
}

// Trace logs SQL queries and their execution time
func (g *GormAdapter) Trace(ctx context.Context, begin time.Time, fc func() (sql string, rowsAffected int64), err error) {
	if g.logLevel <= gormlogger.Silent {
		return
	}

	elapsed := time.Since(begin)
	sql, rows := fc()

	fields := map[string]interface{}{
		"elapsed_ms": float64(elapsed.Nanoseconds()) / 1e6,
		"rows":       rows,
	}

	switch {
	case err != nil && g.logLevel >= gormlogger.Error && (!errors.Is(err, gorm.ErrRecordNotFound) || !g.ignoreRecordNotFoundError):
		fields["sql"] = sql
		g.logger.WithFields(fields).Error("Database query error", err)

	case elapsed > g.slowThreshold && g.slowThreshold != 0 && g.logLevel >= gormlogger.Warn:
		fields["sql"] = sql
		fields["threshold_ms"] = float64(g.slowThreshold.Nanoseconds()) / 1e6
		g.logger.WithFields(fields).Warn("Slow SQL query detected")

	case g.logLevel >= gormlogger.Info:
		fields["sql"] = sql
		g.logger.WithFields(fields).Debug("SQL query executed")
	}
}

// mapToGormLevel maps application log level string to GORM log level
func mapToGormLevel(level string) gormlogger.LogLevel {
	switch level {
	case "debug":
		return gormlogger.Info // Show all SQL queries
	case "info":
		return gormlogger.Warn // Show slow queries and errors
	case "warn":
		return gormlogger.Warn // Show warnings and errors
	case "error":
		return gormlogger.Error // Show errors only
	default:
		return gormlogger.Warn
	}
}
