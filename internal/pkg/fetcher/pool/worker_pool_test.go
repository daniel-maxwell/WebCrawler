package worker_pool

import (
	"context"
	"strings"
	"testing"
	"time"
)

// Checks if a worker pool of a given size can be created without error.
func TestWorkerPool_Success(t *testing.T) {
	poolSize := 2
	workerPool, err := NewWorkerPool(poolSize)
	if err != nil {
		t.Fatalf("Expected NewWorkerPool to succeed, got error: %v", err)
	}
	defer workerPool.Shutdown()

	if len(workerPool.workers) != poolSize {
		t.Errorf("Expected %d workers, got %d", poolSize, len(workerPool.workers))
	}
}

// Checks that shutting down the worker pool does not hang or error.
func TestWorkerPool_Shutdown(t *testing.T) {
	poolSize := 2
	workerPool, err := NewWorkerPool(poolSize)
	if err != nil {
		t.Fatalf("Expected NewWorkerPool to succeed, got error: %v", err)
	}

	// Attempt a clean shutdown
	workerPool.Shutdown()

	channelsClosed := false
	select {
	case <-workerPool.shutdownChannel:
		channelsClosed = true
	case <-time.After(1 * time.Second):
		t.Error("Expected shutdown channel to close, timed out")
	}

	if !channelsClosed {
		t.Error("Expected shutdown channel to close")
	}

	if len(workerPool.workers) != 0 {
		t.Errorf("Expected 0 workers after shutdown, got %d", len(workerPool.workers))
	}
}

// Checks whether a request times out correctly if it takes too long.
func TestWorkerPool_FetchURL_Timeout(t *testing.T) {
	workerPool, err := NewWorkerPool(1)
	if err != nil {
		t.Fatalf("Failed to create WorkerPool: %v", err)
	}
	defer workerPool.Shutdown()

	// Create a very short deadline to force a timeout
	ctx, cancel := context.WithTimeout(context.Background(), 1 * time.Nanosecond)
	defer cancel()

	_, fetchErr := workerPool.FetchURL(ctx, "https://example.com")
	if fetchErr == nil {
		t.Fatal("Expected timeout error, got nil")
	}
	if !strings.Contains(fetchErr.Error(), "deadline exceeded") {
	 	t.Errorf("Expected deadline exceeded error, got: %v", fetchErr)
	}
}
