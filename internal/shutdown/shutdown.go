package shutdown

import (
	"context"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

// Handler manages graceful shutdown of the application
type Handler struct {
	mu             sync.Mutex
	shutdownFuncs  []func(context.Context) error
	timeout        time.Duration
	signalChan     chan os.Signal
	shutdownChan   chan struct{}
	isShuttingDown bool
}

// New creates a new shutdown handler
func New(timeout time.Duration) *Handler {
	return &Handler{
		shutdownFuncs: make([]func(context.Context) error, 0),
		timeout:       timeout,
		signalChan:    make(chan os.Signal, 1),
		shutdownChan:  make(chan struct{}),
	}
}

// Register adds a shutdown function to be called during graceful shutdown
// Functions are called in reverse order of registration (LIFO)
func (h *Handler) Register(fn func(context.Context) error) {
	h.mu.Lock()
	defer h.mu.Unlock()
	h.shutdownFuncs = append(h.shutdownFuncs, fn)
}

// Wait blocks until a shutdown signal is received
func (h *Handler) Wait() {
	signal.Notify(h.signalChan, syscall.SIGINT, syscall.SIGTERM)
	<-h.signalChan
	h.Shutdown()
}

// Shutdown executes all registered shutdown functions with a timeout
func (h *Handler) Shutdown() error {
	h.mu.Lock()
	if h.isShuttingDown {
		h.mu.Unlock()
		return nil
	}
	h.isShuttingDown = true
	h.mu.Unlock()

	close(h.shutdownChan)

	ctx, cancel := context.WithTimeout(context.Background(), h.timeout)
	defer cancel()

	var wg sync.WaitGroup
	errChan := make(chan error, len(h.shutdownFuncs))

	// Execute shutdown functions in reverse order
	for i := len(h.shutdownFuncs) - 1; i >= 0; i-- {
		fn := h.shutdownFuncs[i]
		wg.Add(1)

		go func(shutdownFunc func(context.Context) error) {
			defer wg.Done()
			if err := shutdownFunc(ctx); err != nil {
				errChan <- err
			}
		}(fn)
	}

	// Wait for all shutdown functions to complete or timeout
	done := make(chan struct{})
	go func() {
		wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		close(errChan)
		// Return first error if any
		for err := range errChan {
			return err
		}
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

// IsShuttingDown returns true if shutdown has been initiated
func (h *Handler) IsShuttingDown() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.isShuttingDown
}

// ShutdownChan returns a channel that is closed when shutdown is initiated
func (h *Handler) ShutdownChan() <-chan struct{} {
	return h.shutdownChan
}

// TriggerShutdown programmatically triggers a shutdown
func (h *Handler) TriggerShutdown() {
	select {
	case h.signalChan <- syscall.SIGTERM:
	default:
	}
}
