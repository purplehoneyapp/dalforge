package dal

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestRedisCacheProvider spins up a Redis container and tests the provider.
func TestRedisCacheProvider(t *testing.T) {
	ctx := context.Background()

	// Create a Redis container.
	req := testcontainers.ContainerRequest{
		Image:        "redis:7-alpine", // Use an official Redis image.
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForListeningPort("6379/tcp"),
	}
	redisC, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("failed to start redis container: %v", err)
	}
	// Ensure the container is terminated at the end of the test.
	defer func() {
		if err := redisC.Terminate(ctx); err != nil {
			t.Fatalf("failed to terminate redis container: %v", err)
		}
	}()

	// Get the host and mapped port.
	host, err := redisC.Host(ctx)
	if err != nil {
		t.Fatalf("failed to get container host: %v", err)
	}
	port, err := redisC.MappedPort(ctx, "6379")
	if err != nil {
		t.Fatalf("failed to get container port: %v", err)
	}

	addr := fmt.Sprintf("%s:%s", host, port.Port())
	t.Logf("Address of redis is: %s", addr)

	// Initialize the provider.
	provider := NewRedisCacheProvider(addr, "", 0)
	if err := provider.Connect(); err != nil {
		t.Fatalf("failed to connect to redis: %v", err)
	}
	defer provider.Close()
	t.Logf("Connected to redis: %s", addr)

	// Test OnCacheInvalidated by registering a handler.
	// We'll use a channel to capture the handler invocation.
	resultCh := make(chan string, 1)
	provider.OnCacheInvalidated("test", func(key string) {
		resultCh <- key
	})

	resultCh2 := make(chan string, 1)
	provider.OnCacheInvalidated("test2", func(key string) {
		resultCh2 <- key
	})

	// Test that InvalidateCache returns immediately with no error.
	if err := provider.InvalidateCache("test", "key1"); err != nil {
		t.Errorf("InvalidateCache returned error: %v", err)
	}
	// Test that InvalidateCache returns immediately with no error.
	if err := provider.InvalidateCache("test2", "key2"); err != nil {
		t.Errorf("InvalidateCache returned error: %v", err)
	}

	// Wait for the handler to be invoked.
	receivedCount := 0
loop:
	for {
		select {
		case received := <-resultCh:
			if received != "key1" {
				t.Errorf("expected 'key1', got '%s'", received)
			}
			receivedCount++
			if receivedCount == 2 {
				break loop
			}
		case received := <-resultCh2:
			if received != "key2" {
				t.Errorf("expected 'key2', got '%s'", received)
			}
			receivedCount++
			if receivedCount == 2 {
				break loop
			}
		case <-time.After(5 * time.Second):
			t.Error("timeout waiting for cache invalidation handler")
		}
	}

	// Just test FlushList
	flushResultCh := make(chan string, 1)
	provider.OnCacheFluhsList("test", func() {
		flushResultCh <- "Test"
	})
	if err := provider.FlushListCache("test"); err != nil {
		t.Errorf("FlushListCache returned error: %v", err)
	}

	select {
	case received := <-flushResultCh:
		if received != "Test" {
			t.Errorf("expected 'Test', got '%s'", received)
		}
	case <-time.After(5 * time.Second):
		t.Error("timeout waiting for cache invalidation handler")
	}

}
