package circuitbreaker

import (
	"errors"
	"sync"
	"time"
)

var (
	// ErrOpenState is returned when the circuit breaker is open
	ErrOpenState = errors.New("circuit breaker is open")

	// ErrTooManyRequests is returned when too many requests are made in half-open state
	ErrTooManyRequests = errors.New("too many requests in half-open state")
)

// State represents the circuit breaker state
type State int

const (
	// StateClosed allows all requests through
	StateClosed State = iota

	// StateOpen rejects all requests
	StateOpen

	// StateHalfOpen allows limited requests to test recovery
	StateHalfOpen
)

// String returns the string representation of the state
func (s State) String() string {
	switch s {
	case StateClosed:
		return "closed"
	case StateOpen:
		return "open"
	case StateHalfOpen:
		return "half-open"
	default:
		return "unknown"
	}
}

// Config holds circuit breaker configuration
type Config struct {
	// MaxFailures is the number of failures before opening the circuit
	MaxFailures uint

	// Timeout is how long to wait in open state before moving to half-open
	Timeout time.Duration

	// MaxHalfOpenRequests is the maximum requests allowed in half-open state
	MaxHalfOpenRequests uint

	// IsSuccessful determines if the result is a success
	IsSuccessful func(error) bool
}

// DefaultConfig returns sensible defaults for circuit breaker
func DefaultConfig() Config {
	return Config{
		MaxFailures:         5,
		Timeout:             60 * time.Second,
		MaxHalfOpenRequests: 1,
		IsSuccessful: func(err error) bool {
			return err == nil
		},
	}
}

// CircuitBreaker implements the circuit breaker pattern
type CircuitBreaker struct {
	mu               sync.RWMutex
	state            State
	failures         uint
	successes        uint
	lastStateChange  time.Time
	halfOpenRequests uint
	cfg              Config
}

// New creates a new circuit breaker
func New(cfg Config) *CircuitBreaker {
	if cfg.IsSuccessful == nil {
		cfg.IsSuccessful = func(err error) bool {
			return err == nil
		}
	}

	return &CircuitBreaker{
		state:           StateClosed,
		lastStateChange: time.Now(),
		cfg:             cfg,
	}
}

// Execute runs the given function through the circuit breaker
func (cb *CircuitBreaker) Execute(fn func() error) error {
	if err := cb.beforeRequest(); err != nil {
		return err
	}

	err := fn()
	cb.afterRequest(err)

	return err
}

// beforeRequest checks if the request should be allowed
func (cb *CircuitBreaker) beforeRequest() error {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	switch cb.state {
	case StateClosed:
		return nil

	case StateOpen:
		if time.Since(cb.lastStateChange) > cb.cfg.Timeout {
			cb.setState(StateHalfOpen)
			return nil
		}
		return ErrOpenState

	case StateHalfOpen:
		if cb.halfOpenRequests >= cb.cfg.MaxHalfOpenRequests {
			return ErrTooManyRequests
		}
		cb.halfOpenRequests++
		return nil

	default:
		return ErrOpenState
	}
}

// afterRequest updates the circuit breaker state based on the result
func (cb *CircuitBreaker) afterRequest(err error) {
	cb.mu.Lock()
	defer cb.mu.Unlock()

	if cb.cfg.IsSuccessful(err) {
		cb.onSuccess()
	} else {
		cb.onFailure()
	}
}

// onSuccess handles successful requests
func (cb *CircuitBreaker) onSuccess() {
	switch cb.state {
	case StateClosed:
		cb.failures = 0

	case StateHalfOpen:
		cb.successes++
		if cb.successes >= cb.cfg.MaxHalfOpenRequests {
			cb.setState(StateClosed)
		}
	}
}

// onFailure handles failed requests
func (cb *CircuitBreaker) onFailure() {
	cb.failures++

	switch cb.state {
	case StateClosed:
		if cb.failures >= cb.cfg.MaxFailures {
			cb.setState(StateOpen)
		}

	case StateHalfOpen:
		cb.setState(StateOpen)
	}
}

// setState transitions to a new state
func (cb *CircuitBreaker) setState(state State) {
	cb.state = state
	cb.lastStateChange = time.Now()

	switch state {
	case StateClosed:
		cb.failures = 0
		cb.successes = 0
		cb.halfOpenRequests = 0

	case StateOpen:
		cb.successes = 0
		cb.halfOpenRequests = 0

	case StateHalfOpen:
		cb.successes = 0
		cb.halfOpenRequests = 0
	}
}

// State returns the current state
func (cb *CircuitBreaker) State() State {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.state
}

// Failures returns the current failure count
func (cb *CircuitBreaker) Failures() uint {
	cb.mu.RLock()
	defer cb.mu.RUnlock()
	return cb.failures
}

// Reset resets the circuit breaker to closed state
func (cb *CircuitBreaker) Reset() {
	cb.mu.Lock()
	defer cb.mu.Unlock()
	cb.setState(StateClosed)
}
