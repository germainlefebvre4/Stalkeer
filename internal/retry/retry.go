package retry

import (
	"context"
	"math"
	"math/rand"
	"time"
)

// Config holds retry configuration
type Config struct {
	MaxAttempts       int
	InitialBackoff    time.Duration
	MaxBackoff        time.Duration
	BackoffMultiplier float64
	JitterFraction    float64
}

// DefaultConfig returns sensible defaults for retry configuration
func DefaultConfig() Config {
	return Config{
		MaxAttempts:       3,
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        30 * time.Second,
		BackoffMultiplier: 2.0,
		JitterFraction:    0.1,
	}
}

// IsRetryable is a function that determines if an error should trigger a retry
type IsRetryable func(error) bool

// Do executes the given function with exponential backoff retry logic
func Do(ctx context.Context, cfg Config, fn func() error, isRetryable IsRetryable) error {
	var err error
	backoff := cfg.InitialBackoff

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		err = fn()
		if err == nil {
			return nil
		}

		// Check if error is retryable
		if !isRetryable(err) {
			return err
		}

		// Don't sleep after last attempt
		if attempt == cfg.MaxAttempts {
			return err
		}

		// Calculate backoff with jitter
		sleep := calculateBackoff(backoff, cfg.JitterFraction)

		// Check context cancellation before sleeping
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(sleep):
		}

		// Increase backoff for next iteration
		backoff = time.Duration(float64(backoff) * cfg.BackoffMultiplier)
		if backoff > cfg.MaxBackoff {
			backoff = cfg.MaxBackoff
		}
	}

	return err
}

// DoWithResult executes the given function with exponential backoff retry logic
// and returns the result along with any error
func DoWithResult[T any](ctx context.Context, cfg Config, fn func() (T, error), isRetryable IsRetryable) (T, error) {
	var result T
	var err error
	backoff := cfg.InitialBackoff

	for attempt := 1; attempt <= cfg.MaxAttempts; attempt++ {
		result, err = fn()
		if err == nil {
			return result, nil
		}

		// Check if error is retryable
		if !isRetryable(err) {
			return result, err
		}

		// Don't sleep after last attempt
		if attempt == cfg.MaxAttempts {
			return result, err
		}

		// Calculate backoff with jitter
		sleep := calculateBackoff(backoff, cfg.JitterFraction)

		// Check context cancellation before sleeping
		select {
		case <-ctx.Done():
			return result, ctx.Err()
		case <-time.After(sleep):
		}

		// Increase backoff for next iteration
		backoff = time.Duration(float64(backoff) * cfg.BackoffMultiplier)
		if backoff > cfg.MaxBackoff {
			backoff = cfg.MaxBackoff
		}
	}

	return result, err
}

// calculateBackoff adds jitter to prevent thundering herd
func calculateBackoff(backoff time.Duration, jitterFraction float64) time.Duration {
	if jitterFraction <= 0 {
		return backoff
	}

	jitter := float64(backoff) * jitterFraction
	randomJitter := (rand.Float64()*2 - 1) * jitter

	result := float64(backoff) + randomJitter
	if result < 0 {
		result = 0
	}

	return time.Duration(result)
}

// Backoff calculates the backoff duration for a given attempt
func Backoff(attempt int, cfg Config) time.Duration {
	if attempt <= 0 {
		return 0
	}

	backoff := float64(cfg.InitialBackoff) * math.Pow(cfg.BackoffMultiplier, float64(attempt-1))
	duration := time.Duration(backoff)

	if duration > cfg.MaxBackoff {
		duration = cfg.MaxBackoff
	}

	return calculateBackoff(duration, cfg.JitterFraction)
}
