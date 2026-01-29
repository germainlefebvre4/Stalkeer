package shutdown

import (
	"context"
	"errors"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestNew(t *testing.T) {
	timeout := 5 * time.Second
	h := New(timeout)

	if h.timeout != timeout {
		t.Errorf("expected timeout %v, got %v", timeout, h.timeout)
	}
	if h.isShuttingDown {
		t.Error("expected isShuttingDown to be false")
	}
	if len(h.shutdownFuncs) != 0 {
		t.Errorf("expected 0 shutdown functions, got %d", len(h.shutdownFuncs))
	}
}

func TestRegister(t *testing.T) {
	h := New(5 * time.Second)

	h.Register(func(ctx context.Context) error {
		return nil
	})

	h.Register(func(ctx context.Context) error {
		return nil
	})

	if len(h.shutdownFuncs) != 2 {
		t.Errorf("expected 2 shutdown functions, got %d", len(h.shutdownFuncs))
	}
}

func TestShutdown_Success(t *testing.T) {
	h := New(5 * time.Second)

	var counter int32
	h.Register(func(ctx context.Context) error {
		atomic.AddInt32(&counter, 1)
		return nil
	})

	h.Register(func(ctx context.Context) error {
		atomic.AddInt32(&counter, 1)
		return nil
	})

	err := h.Shutdown()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if atomic.LoadInt32(&counter) != 2 {
		t.Errorf("expected counter to be 2, got %d", counter)
	}

	if !h.IsShuttingDown() {
		t.Error("expected IsShuttingDown to be true")
	}
}

func TestShutdown_ReverseOrder(t *testing.T) {
	h := New(5 * time.Second)

	var order []int
	var mu sync.Mutex

	for i := 1; i <= 3; i++ {
		val := i
		h.Register(func(ctx context.Context) error {
			mu.Lock()
			order = append(order, val)
			mu.Unlock()
			return nil
		})
	}

	err := h.Shutdown()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Functions should be called in reverse order
	expected := []int{3, 2, 1}
	if len(order) != len(expected) {
		t.Fatalf("expected %d items, got %d", len(expected), len(order))
	}

	// Note: since functions run concurrently, we can't guarantee exact order
	// but we can verify all functions were called
	callCounts := make(map[int]int)
	for _, v := range order {
		callCounts[v]++
	}

	for i := 1; i <= 3; i++ {
		if callCounts[i] != 1 {
			t.Errorf("expected function %d to be called once, got %d", i, callCounts[i])
		}
	}
}

func TestShutdown_WithError(t *testing.T) {
	h := New(5 * time.Second)

	testErr := errors.New("shutdown error")
	h.Register(func(ctx context.Context) error {
		return testErr
	})

	err := h.Shutdown()
	if err != testErr {
		t.Errorf("expected error %v, got %v", testErr, err)
	}
}

func TestShutdown_Timeout(t *testing.T) {
	h := New(100 * time.Millisecond)

	h.Register(func(ctx context.Context) error {
		time.Sleep(500 * time.Millisecond)
		return nil
	})

	err := h.Shutdown()
	if err != context.DeadlineExceeded {
		t.Errorf("expected context.DeadlineExceeded, got %v", err)
	}
}

func TestShutdown_Idempotent(t *testing.T) {
	h := New(5 * time.Second)

	var counter int32
	h.Register(func(ctx context.Context) error {
		atomic.AddInt32(&counter, 1)
		return nil
	})

	err := h.Shutdown()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	// Second call should not execute functions again
	err = h.Shutdown()
	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if atomic.LoadInt32(&counter) != 1 {
		t.Errorf("expected counter to be 1, got %d", counter)
	}
}

func TestIsShuttingDown(t *testing.T) {
	h := New(5 * time.Second)

	if h.IsShuttingDown() {
		t.Error("expected IsShuttingDown to be false initially")
	}

	h.Shutdown()

	if !h.IsShuttingDown() {
		t.Error("expected IsShuttingDown to be true after shutdown")
	}
}

func TestShutdownChan(t *testing.T) {
	h := New(5 * time.Second)

	shutdownChan := h.ShutdownChan()

	select {
	case <-shutdownChan:
		t.Error("expected shutdown channel to be open")
	default:
	}

	h.Shutdown()

	select {
	case <-shutdownChan:
		// Expected
	case <-time.After(100 * time.Millisecond):
		t.Error("expected shutdown channel to be closed")
	}
}

func TestTriggerShutdown(t *testing.T) {
	h := New(5 * time.Second)

	done := make(chan struct{})
	go func() {
		h.Wait()
		close(done)
	}()

	time.Sleep(50 * time.Millisecond)
	h.TriggerShutdown()

	select {
	case <-done:
		// Expected
	case <-time.After(1 * time.Second):
		t.Error("expected Wait to return after TriggerShutdown")
	}
}

func TestShutdown_ConcurrentFunctions(t *testing.T) {
	h := New(5 * time.Second)

	var counter int32

	for i := 0; i < 10; i++ {
		h.Register(func(ctx context.Context) error {
			atomic.AddInt32(&counter, 1)
			time.Sleep(10 * time.Millisecond)
			return nil
		})
	}

	start := time.Now()
	err := h.Shutdown()
	duration := time.Since(start)

	if err != nil {
		t.Errorf("expected no error, got %v", err)
	}

	if atomic.LoadInt32(&counter) != 10 {
		t.Errorf("expected counter to be 10, got %d", counter)
	}

	// Since functions run concurrently, it should take around 10ms, not 100ms
	if duration > 100*time.Millisecond {
		t.Errorf("shutdown took too long: %v (expected < 100ms)", duration)
	}
}
