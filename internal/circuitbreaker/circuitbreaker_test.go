package circuitbreaker

import (
	"errors"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	cfg := DefaultConfig()
	cb := New(cfg)

	if cb.state != StateClosed {
		t.Errorf("expected state Closed, got %s", cb.state)
	}
	if cb.failures != 0 {
		t.Errorf("expected 0 failures, got %d", cb.failures)
	}
}

func TestDefaultConfig(t *testing.T) {
	cfg := DefaultConfig()

	if cfg.MaxFailures != 5 {
		t.Errorf("expected MaxFailures 5, got %d", cfg.MaxFailures)
	}
	if cfg.Timeout != 60*time.Second {
		t.Errorf("expected Timeout 60s, got %v", cfg.Timeout)
	}
	if cfg.MaxHalfOpenRequests != 1 {
		t.Errorf("expected MaxHalfOpenRequests 1, got %d", cfg.MaxHalfOpenRequests)
	}
}

func TestExecute_Success(t *testing.T) {
	cfg := DefaultConfig()
	cb := New(cfg)

	err := cb.Execute(func() error {
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if cb.State() != StateClosed {
		t.Errorf("expected state Closed, got %s", cb.State())
	}
}

func TestExecute_Failure(t *testing.T) {
	cfg := DefaultConfig()
	cb := New(cfg)

	testErr := errors.New("test error")
	err := cb.Execute(func() error {
		return testErr
	})

	if err != testErr {
		t.Errorf("expected error %v, got %v", testErr, err)
	}
	if cb.Failures() != 1 {
		t.Errorf("expected 1 failure, got %d", cb.Failures())
	}
}

func TestCircuitBreaker_OpensAfterMaxFailures(t *testing.T) {
	cfg := Config{
		MaxFailures:         3,
		Timeout:             1 * time.Second,
		MaxHalfOpenRequests: 1,
		IsSuccessful: func(err error) bool {
			return err == nil
		},
	}
	cb := New(cfg)

	testErr := errors.New("test error")

	// Execute failing requests
	for i := 0; i < 3; i++ {
		cb.Execute(func() error {
			return testErr
		})
	}

	if cb.State() != StateOpen {
		t.Errorf("expected state Open after max failures, got %s", cb.State())
	}

	// Next request should be rejected
	err := cb.Execute(func() error {
		return nil
	})

	if err != ErrOpenState {
		t.Errorf("expected ErrOpenState, got %v", err)
	}
}

func TestCircuitBreaker_HalfOpenAfterTimeout(t *testing.T) {
	cfg := Config{
		MaxFailures:         2,
		Timeout:             100 * time.Millisecond,
		MaxHalfOpenRequests: 1,
		IsSuccessful: func(err error) bool {
			return err == nil
		},
	}
	cb := New(cfg)

	// Open the circuit
	testErr := errors.New("test error")
	for i := 0; i < 2; i++ {
		cb.Execute(func() error {
			return testErr
		})
	}

	if cb.State() != StateOpen {
		t.Fatalf("expected state Open, got %s", cb.State())
	}

	// Wait for timeout
	time.Sleep(150 * time.Millisecond)

	// Next request should transition to half-open
	cb.Execute(func() error {
		return nil
	})

	if cb.State() != StateClosed {
		t.Errorf("expected state Closed after successful half-open request, got %s", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenSuccess(t *testing.T) {
	cfg := Config{
		MaxFailures:         2,
		Timeout:             50 * time.Millisecond,
		MaxHalfOpenRequests: 1,
		IsSuccessful: func(err error) bool {
			return err == nil
		},
	}
	cb := New(cfg)

	// Open the circuit
	testErr := errors.New("test error")
	for i := 0; i < 2; i++ {
		cb.Execute(func() error {
			return testErr
		})
	}

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// Successful request in half-open should close the circuit
	err := cb.Execute(func() error {
		return nil
	})

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}
	if cb.State() != StateClosed {
		t.Errorf("expected state Closed, got %s", cb.State())
	}
}

func TestCircuitBreaker_HalfOpenFailure(t *testing.T) {
	cfg := Config{
		MaxFailures:         2,
		Timeout:             50 * time.Millisecond,
		MaxHalfOpenRequests: 1,
		IsSuccessful: func(err error) bool {
			return err == nil
		},
	}
	cb := New(cfg)

	// Open the circuit
	testErr := errors.New("test error")
	for i := 0; i < 2; i++ {
		cb.Execute(func() error {
			return testErr
		})
	}

	// Wait for timeout
	time.Sleep(100 * time.Millisecond)

	// Failed request in half-open should reopen the circuit
	cb.Execute(func() error {
		return testErr
	})

	if cb.State() != StateOpen {
		t.Errorf("expected state Open after half-open failure, got %s", cb.State())
	}
}

func TestCircuitBreaker_TooManyHalfOpenRequests(t *testing.T) {
	cfg := Config{
		MaxFailures:         2,
		Timeout:             50 * time.Millisecond,
		MaxHalfOpenRequests: 1,
		IsSuccessful: func(err error) bool {
			return err == nil
		},
	}
	cb := New(cfg)

	// Open the circuit
	testErr := errors.New("test error")
	for i := 0; i < 2; i++ {
		cb.Execute(func() error {
			return testErr
		})
	}

	// Wait for timeout to transition to half-open
	time.Sleep(100 * time.Millisecond)

	// Lock to manually test half-open state
	cb.mu.Lock()
	cb.state = StateHalfOpen
	cb.halfOpenRequests = 1 // Already at limit
	cb.mu.Unlock()

	// Next request should be rejected
	err := cb.beforeRequest()
	if err != ErrTooManyRequests {
		t.Errorf("expected ErrTooManyRequests, got %v", err)
	}
}

func TestCircuitBreaker_Reset(t *testing.T) {
	cfg := DefaultConfig()
	cb := New(cfg)

	// Open the circuit
	testErr := errors.New("test error")
	for i := 0; i < int(cfg.MaxFailures); i++ {
		cb.Execute(func() error {
			return testErr
		})
	}

	if cb.State() != StateOpen {
		t.Fatalf("expected state Open, got %s", cb.State())
	}

	// Reset the circuit breaker
	cb.Reset()

	if cb.State() != StateClosed {
		t.Errorf("expected state Closed after reset, got %s", cb.State())
	}
	if cb.Failures() != 0 {
		t.Errorf("expected 0 failures after reset, got %d", cb.Failures())
	}
}

func TestState_String(t *testing.T) {
	tests := []struct {
		state    State
		expected string
	}{
		{StateClosed, "closed"},
		{StateOpen, "open"},
		{StateHalfOpen, "half-open"},
		{State(999), "unknown"},
	}

	for _, tt := range tests {
		if got := tt.state.String(); got != tt.expected {
			t.Errorf("State.String() = %v, want %v", got, tt.expected)
		}
	}
}

func TestCircuitBreaker_CustomIsSuccessful(t *testing.T) {
	customErr := errors.New("custom retryable error")

	cfg := Config{
		MaxFailures:         3,
		Timeout:             1 * time.Second,
		MaxHalfOpenRequests: 1,
		IsSuccessful: func(err error) bool {
			// Treat customErr as success
			return err == nil || err == customErr
		},
	}
	cb := New(cfg)

	// Execute with custom error (should be treated as success)
	for i := 0; i < 5; i++ {
		cb.Execute(func() error {
			return customErr
		})
	}

	if cb.State() != StateClosed {
		t.Errorf("expected state Closed with custom IsSuccessful, got %s", cb.State())
	}

	// Execute with different error (should fail)
	otherErr := errors.New("other error")
	for i := 0; i < 3; i++ {
		cb.Execute(func() error {
			return otherErr
		})
	}

	if cb.State() != StateOpen {
		t.Errorf("expected state Open after real failures, got %s", cb.State())
	}
}
