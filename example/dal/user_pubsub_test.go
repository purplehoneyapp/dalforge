package dal

import (
	"context"
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/sony/gobreaker"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

// FakeCacheProvider is a fake implementation of CacheProvider for testing pub/sub.
type FakeCacheProvider struct {
	invalidationCallbacks map[string]func(string)
	flushCallbacks        map[string]func()
}

func NewFakeCacheProvider() *FakeCacheProvider {
	return &FakeCacheProvider{
		invalidationCallbacks: make(map[string]func(string)),
		flushCallbacks:        make(map[string]func()),
	}
}

// InvalidateCache calls the registered invalidation callback for the entity.
func (f *FakeCacheProvider) InvalidateCache(entityName, cacheKey string) error {
	if callback, ok := f.invalidationCallbacks[entityName]; ok {
		callback(cacheKey)
	}
	return nil
}

func (f *FakeCacheProvider) Close() {
}
func (f *FakeCacheProvider) Connect() error { return nil }

// OnCacheInvalidated stores the callback.
func (f *FakeCacheProvider) OnCacheInvalidated(entityName string, handler func(string)) {
	f.invalidationCallbacks[entityName] = handler
}

// OnCacheFlushList stores the flush callback.
func (f *FakeCacheProvider) OnCacheFlushList(entityName string, handler func()) {
	f.flushCallbacks[entityName] = handler
}

// FlushListCache calls the registered flush callback.
func (f *FakeCacheProvider) FlushListCache(entityName string) error {
	if callback, ok := f.flushCallbacks[entityName]; ok {
		callback()
	}
	return nil
}

// SimulateCacheInvalidation is a helper to simulate a pub/sub invalidation event.
func (f *FakeCacheProvider) SimulateCacheInvalidation(entityName, cacheKey string) {
	if callback, ok := f.invalidationCallbacks[entityName]; ok {
		callback(cacheKey)
	}
}

func TestPubSubCacheInvalidation(t *testing.T) {
	t.Run("TestPubSubCacheInvalidation", func(t *testing.T) {
		setupTestDB(t)
		defer teardownTestDB(t)

		// Create a fake cache provider that will capture invalidation calls.
		fakeCacheProv := NewFakeCacheProvider()

		// Initialize DAL with the fake cache provider.
		userDAL := NewUserDAL(
			dbProvider,
			fakeCacheProv,
			nil, // Use default config provider
			gobreaker.Settings{},
		)
		ctx := context.Background()

		// Create a user so that it is cached.
		newUser := &User{
			Age:       25,
			Email:     "pubsub_test@example.com",
			Uuid:      uuid.New().String(),
			Status:    sql.NullString{String: "active", Valid: true},
			Birthdate: sql.NullTime{Time: time.Now(), Valid: true},
		}
		created, err := userDAL.Create(ctx, newUser)
		assert.NoError(t, err)
		assert.NotZero(t, created.ID)

		// Force caching by performing a GetByID.
		cachedUser, err := userDAL.GetByID(ctx, created.ID)
		assert.NoError(t, err)
		assert.NotNil(t, cachedUser)

		// The local cache should contain the user.
		// We can directly check using the internal getByIDCached.
		cachedBefore, _ := userDAL.getByIDCached(created.ID)
		assert.NotNil(t, cachedBefore, "Expected local cache to contain the user before invalidation")

		// Now simulate a pub/sub invalidation event.
		cacheKey := userDAL.getCacheKey(created.ID)
		fakeCacheProv.SimulateCacheInvalidation("user", cacheKey)

		// After invalidation, the local cache should be cleared.
		cachedAfter, _ := userDAL.getByIDCached(created.ID)
		assert.Nil(t, cachedAfter, "Expected local cache to be cleared after pub/sub invalidation")
	})
}

func TestRedisPubSubInvalidationBetweenInstances(t *testing.T) {
	ctx := context.Background()

	// Launch a Redis container.
	req := testcontainers.ContainerRequest{
		Image:        "redis:7.0",
		ExposedPorts: []string{"6379/tcp"},
		WaitingFor:   wait.ForListeningPort("6379/tcp"),
	}
	redisContainer, err := testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		t.Fatalf("Failed to start Redis container: %v", err)
	}
	defer func() {
		if err := redisContainer.Terminate(ctx); err != nil {
			t.Logf("Failed to terminate Redis container: %v", err)
		}
	}()

	host, err := redisContainer.Host(ctx)
	if err != nil {
		t.Fatalf("Failed to get Redis host: %v", err)
	}
	port, err := redisContainer.MappedPort(ctx, "6379")
	if err != nil {
		t.Fatalf("Failed to get Redis port: %v", err)
	}
	redisAddr := fmt.Sprintf("%s:%s", host, port.Port())

	// Create two separate RedisCacheProviders (simulating two instances).
	redisCacheProvA := NewRedisCacheProvider(redisAddr, "", 0)
	err = redisCacheProvA.Connect()
	if err != nil {
		t.Fatalf("Instance A: failed to connect to Redis: %v", err)
	}
	defer redisCacheProvA.Close()

	redisCacheProvB := NewRedisCacheProvider(redisAddr, "", 0)
	err = redisCacheProvB.Connect()
	if err != nil {
		t.Fatalf("Instance B: failed to connect to Redis: %v", err)
	}
	defer redisCacheProvB.Close()

	// Set up a fresh test DB.
	setupTestDB(t)
	defer teardownTestDB(t)

	// Create two separate UserDAL instances using the same DBProvider but different cache providers.
	userDAL_A := NewUserDAL(dbProvider, redisCacheProvA, nil, gobreaker.Settings{})
	userDAL_B := NewUserDAL(dbProvider, redisCacheProvB, nil, gobreaker.Settings{})

	// Give the subscriptions some time to initialize.
	time.Sleep(3 * time.Second)

	// --- Create a user using instance A.
	newUser := &User{
		Age:       25,
		Email:     "instance_test@example.com",
		Uuid:      uuid.New().String(),
		Status:    sql.NullString{String: "active", Valid: true},
		Birthdate: sql.NullTime{Time: time.Now(), Valid: true},
	}
	created, err := userDAL_A.Create(ctx, newUser)
	assert.NoError(t, err)
	assert.NotZero(t, created.ID)

	// --- Retrieve the user using instance B so that it caches the record locally.
	userB, err := userDAL_B.GetByID(ctx, created.ID)
	assert.NoError(t, err)
	assert.Equal(t, created.Email, userB.Email)

	// --- Update the user using instance A.
	updatedEmail := "updated_instance@example.com"
	created.Email = updatedEmail
	err = userDAL_A.Update(ctx, created)
	assert.NoError(t, err)

	// Allow time for the pub/sub invalidation to propagate to instance B.
	time.Sleep(3 * time.Second)

	// Now, instance B should no longer have the stale cached record.
	userBUpdated, err := userDAL_B.GetByID(ctx, created.ID)
	assert.NoError(t, err)
	assert.Equal(t, updatedEmail, userBUpdated.Email)

	// --- Delete the user using instance A.
	err = userDAL_A.Delete(ctx, created)
	assert.NoError(t, err)

	// Allow time for deletion invalidation to propagate.
	time.Sleep(3 * time.Second)

	// Instance B should now return ErrNotFound.
	_, err = userDAL_B.GetByID(ctx, created.ID)
	assert.Error(t, err)
	assert.Equal(t, ErrNotFound, err)
}
