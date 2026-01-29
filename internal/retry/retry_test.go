package retry

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDo_Success(t *testing.T) {
	cfg := DefaultConfig()
	attempts := 0

	err := Do(context.Background(), cfg, func() error {
		attempts++
		return nil
	}, func(err error) bool {
		return true
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestDo_SuccessAfterRetries(t *testing.T) {
	cfg := Config{
		MaxAttempts:       3,
		InitialBackoff:    1 * time.Millisecond,
		MaxBackoff:        10 * time.Millisecond,
		BackoffMultiplier: 2.0,
		JitterFraction:    0,
	}

	attempts := 0
	err := Do(context.Background(), cfg, func() error {
		attempts++
		if attempts < 3 {
			return errors.New("temporary error")
		}
		return nil
	}, func(err error) bool {
		return true
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestDo_NonRetryableError(t *testing.T) {
	cfg := DefaultConfig()
	testErr := errors.New("non-retryable")
	attempts := 0

	err := Do(context.Background(), cfg, func() error {
		attempts++
		return testErr
	}, func(err error) bool {
		return false
	})

	if err != testErr {
		t.Errorf("expected error %v, got %v", testErr, err)
	}
	if attempts != 1 {
		t.Errorf("expected 1 attempt, got %d", attempts)
	}
}

func TestDo_MaxAttemptsExceeded(t *testing.T) {
	cfg := Config{
		MaxAttempts:       3,
		InitialBackoff:    1 * time.Millisecond,
		MaxBackoff:        10 * time.Millisecond,
		BackoffMultiplier: 2.0,
		JitterFraction:    0,
	}

	testErr := errors.New("persistent error")
	attempts := 0

	err := Do(context.Background(), cfg, func() error {
		attempts++
		return testErr
	}, func(err error) bool {
		return true
	})

	if err != testErr {
		t.Errorf("expected error %v, got %v", testErr, err)
	}
	if attempts != cfg.MaxAttempts {
		t.Errorf("expected %d attempts, got %d", cfg.MaxAttempts, attempts)
	}
}

func TestDo_ContextCancellation(t *testing.T) {
	cfg := Config{
		MaxAttempts:       5,
		InitialBackoff:    50 * time.Millisecond,
		MaxBackoff:        100 * time.Millisecond,
		BackoffMultiplier: 2.0,
		JitterFraction:    0,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()

	attempts := 0
	err := Do(ctx, cfg, func() error {
		attempts++
		return errors.New("retryable error")
	}, func(err error) bool {
		return true
	})

	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
	if attempts >= cfg.MaxAttempts {
		t.Errorf("expected fewer than %d attempts due to context cancellation, got %d", cfg.MaxAttempts, attempts)
	}
}

func TestDoWithResult_Success(t *testing.T) {
	cfg := DefaultConfig()

	result, err := DoWithResult(context.Background(), cfg, func() (string, error) {
		return "success", nil
	}, func(err error) bool {
		return true
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != "success" {
		t.Errorf("expected result 'success', got %s", result)
	}
}

func TestDoWithResult_SuccessAfterRetries(t *testing.T) {
	cfg := Config{
		MaxAttempts:       3,
		InitialBackoff:    1 * time.Millisecond,
		MaxBackoff:        10 * time.Millisecond,
		BackoffMultiplier: 2.0,
		JitterFraction:    0,
	}

	attempts := 0
	result, err := DoWithResult(context.Background(), cfg, func() (int, error) {
		attempts++
		if attempts < 3 {
			return 0, errors.New("temporary error")
		}
		return 42, nil
	}, func(err error) bool {
		return true
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if result != 42 {
		t.Errorf("expected result 42, got %d", result)
	}
	if attempts != 3 {
		t.Errorf("expected 3 attempts, got %d", attempts)
	}
}

func TestDoWithResult_NonRetryableError(t *testing.T) {
	cfg := DefaultConfig()
	testErr := errors.New("non-retryable")

	result, err := DoWithResult(context.Background(), cfg, func() (string, error) {
		return "", testErr
	}, func(err error) bool {
		return false
	})

	if err != testErr {
		t.Errorf("expected error %v, got %v", testErr, err)
	}
	if result != "" {
		t.Errorf("expected empty result, got %s", result)
	}
}

func TestCalculateBackoff(t *testing.T) {
	backoff := 100 * time.Millisecond

	t.Run("no jitter", func(t *testing.T) {
		result := calculateBackoff(backoff, 0)
		if result != backoff {
			t.Errorf("expected %v, got %v", backoff, result)
		}
	})

	t.Run("with jitter", func(t *testing.T) {
		jitterFraction := 0.1
		for i := 0; i < 100; i++ {
			result := calculateBackoff(backoff, jitterFraction)
			minExpected := time.Duration(float64(backoff) * (1 - jitterFraction))
			maxExpected := time.Duration(float64(backoff) * (1 + jitterFraction))

			if result < minExpected || result > maxExpected {
				t.Errorf("expected result between %v and %v, got %v", minExpected, maxExpected, result)
			}
		}
	})
}

func TestBackoff(t *testing.T) {
	cfg := Config{
		InitialBackoff:    100 * time.Millisecond,
		MaxBackoff:        1 * time.Second,
		BackoffMultiplier: 2.0,
		JitterFraction:    0,
	}

	tests := []struct {
		attempt  int
		expected time.Duration
	}{
		{0, 0},
		{1, 100 * time.Millisecond},
		{2, 200 * time.Millisecond},
		{3, 400 * time.Millisecond},
		{4, 800 * time.Millisecond},
		{5, 1 * time.Second}, // capped at MaxBackoff
		{6, 1 * time.Second}, // capped at MaxBackoff
	}

	for _, tt := range tests {
		result := Backoff(tt.attempt, cfg)
		if result != tt.expected {
			t.Errorf("Backoff(%d) = %v, want %v", tt.attempt, result, tt.expected)
		}
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxAttempts != 3 {
		t.Errorf("expected MaxAttempts 3, got %d", cfg.MaxAttempts)
	}
	if cfg.InitialBackoff != 100*time.Millisecond {
		t.Errorf("expected InitialBackoff 100ms, got %v", cfg.InitialBackoff)
	}
	if cfg.MaxBackoff != 30*time.Second {
		t.Errorf("expected MaxBackoff 30s, got %v", cfg.MaxBackoff)
	}
	if cfg.BackoffMultiplier != 2.0 {
		t.Errorf("expected BackoffMultiplier 2.0, got %f", cfg.BackoffMultiplier)
	}
	if cfg.JitterFraction != 0.1 {
		t.Errorf("expected JitterFraction 0.1, got %f", cfg.JitterFraction)
	}
}
