package dal

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/sony/gobreaker"
	"github.com/stretchr/testify/assert"
)

// FailingDBProvider always returns an error.
type FailingDBProvider struct{}

func (p FailingDBProvider) GetDatabase(_ string, _ bool) (*sql.DB, error) {
	return nil, fmt.Errorf("simulated DB failure")
}
func (p FailingDBProvider) Connect() error    { return nil }
func (p FailingDBProvider) Disconnect() error { return nil }

func TestCircuitBreaker(t *testing.T) {
	// Use a short timeout and trip after 2 consecutive failures to speed up the test.
	settings := gobreaker.Settings{
		Name: "test_cb",
		ReadyToTrip: func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 1
		},
		Timeout: 2 * time.Second,
	}

	// Instantiate the UserDAL with the failing provider, using NoopCacheProvider and DefaultConfigProvider.
	failingProvider := FailingDBProvider{}
	userDAL := NewUserDAL(failingProvider, NoopCacheProvider{}, DefaultConfigProvider{}, settings)
	ctx := context.Background()

	// Call GetByID repeatedly to force failures.
	for i := 0; i < 3; i++ {
		_, err := userDAL.GetByID(ctx, 1)
		assert.Error(t, err, "Expected error from failing DBProvider")
	}

	// Now the circuit breaker should have tripped (i.e. state open).
	state := userDAL.dbBreaker.State()
	assert.Equal(t, gobreaker.StateOpen, state, "Expected circuit breaker to be open after consecutive failures")

	// Optionally wait for the timeout period so the breaker transitions toward half-open.
	time.Sleep(3 * time.Second)
	_, err := userDAL.GetByID(ctx, 1)
	assert.Error(t, err, "Expected error on trial call in half-open state")
	stateAfter := userDAL.dbBreaker.State()
	t.Logf("Circuit breaker state after timeout: %v", stateAfter)
}

func TestCircuitBreakerWithErrNotFound(t *testing.T) {
	t.Run("TestCircuitBreakerWithErrNotFound", func(t *testing.T) {
		// Set up a fresh test database.
		setupTestDB(t)
		defer teardownTestDB(t)

		settings := gobreaker.Settings{
			Name: "test_cb",
			ReadyToTrip: func(counts gobreaker.Counts) bool {
				return counts.ConsecutiveFailures > 1
			},
			Timeout: 2 * time.Second,
		}

		// Initialize DAL with default providers.
		userDAL := NewUserDAL(
			dbProvider,
			nil, // default cache provider
			nil, // default config provider
			settings,
		)
		ctx := context.Background()

		// Create a user.
		res, err := userDAL.getByID(ctx, 10000)
		assert.ErrorIs(t, err, ErrNotFound)
		assert.Nil(t, res)

		res2, err2 := userDAL.getByID(ctx, 10001)
		assert.ErrorIs(t, err2, ErrNotFound)
		assert.Nil(t, res2)

		state := userDAL.dbBreaker.State()
		assert.Equal(t, gobreaker.StateClosed, state, "Expected circuit breaker to be closed after ErrNotFound")
	})
}
