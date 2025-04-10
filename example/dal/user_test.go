// user_test.gen.go
package dal

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/testutil"
	"github.com/sony/gobreaker"
	"github.com/stretchr/testify/assert"
	"github.com/testcontainers/testcontainers-go"
	"github.com/testcontainers/testcontainers-go/wait"
)

var (
	mysqlContainer testcontainers.Container
	mainDbProvider *TestDBProvider
	dbProvider     *TestDBProvider
)

type TestDBProvider struct {
	connStr    string
	connection *sql.DB
}

func (p *TestDBProvider) GetDatabase(_ string, _ bool) (*sql.DB, error) {
	return p.connection, nil
}
func (p *TestDBProvider) AllDatabases(_ string, _ string) []*sql.DB {
	return nil
}
func (p *TestDBProvider) Connect() error {
	conn, err := sql.Open("mysql", p.connStr)
	if err != nil {
		return err
	}
	// ping connection
	if err = conn.Ping(); err != nil {
		return fmt.Errorf("failed to ping connection: %w", err)
	}

	p.connection = conn
	return nil
}
func (p *TestDBProvider) Disconnect() error {
	p.connection.Close()
	p.connection = nil
	return nil
}

func TestMain(m *testing.M) {
	ctx := context.Background()

	// Start MySQL container
	req := testcontainers.ContainerRequest{
		Image:        "mysql:8.0",
		ExposedPorts: []string{"3306/tcp"},
		Env: map[string]string{
			"MYSQL_ROOT_PASSWORD": "root",
			"MYSQL_DATABASE":      "test",
		},
		WaitingFor: wait.ForLog("port: 3306  MySQL Community Server"),
	}

	var err error
	mysqlContainer, err = testcontainers.GenericContainer(ctx, testcontainers.GenericContainerRequest{
		ContainerRequest: req,
		Started:          true,
	})
	if err != nil {
		log.Fatalf("Failed to start container: %v", err)
	}
	defer mysqlContainer.Terminate(ctx)

	// Get connection details
	host, _ := mysqlContainer.Host(ctx)
	port, _ := mysqlContainer.MappedPort(ctx, "3306")

	connStr := fmt.Sprintf("root:root@tcp(%s:%s)/test?parseTime=true&multiStatements=true", host, port.Port())
	mainDbProvider = &TestDBProvider{connStr: connStr}
	err = mainDbProvider.Connect()
	if err != nil {
		log.Fatalf("failed connecting to mysql: %v", err)
	}

	// Run tests
	code := m.Run()
	os.Exit(code)
}

func setupTestDB(t *testing.T) {
	ResetTelemetry()
	registry := prometheus.NewRegistry()
	RegisterTelemetry(registry)

	ctx := context.Background()
	db, err := mainDbProvider.GetDatabase("", false)
	if err != nil {
		t.Fatal(err)
	}

	// Create test database
	dbName := fmt.Sprintf("test_%s", strings.ReplaceAll(uuid.New().String(), "-", "_"))
	_, err = db.ExecContext(ctx, fmt.Sprintf("CREATE DATABASE %s", dbName))
	if err != nil {
		t.Fatal(err)
	}

	// Preserve original host:port from container
	originalConnStr := mainDbProvider.connStr
	parts := strings.Split(originalConnStr, "/")
	hostPortPart := parts[0] // "root:root@tcp(host:port)"

	connStr := fmt.Sprintf("%s/%s?parseTime=true&multiStatements=true", hostPortPart, dbName)
	dbProvider = &TestDBProvider{connStr: connStr}
	err = dbProvider.Connect()
	if err != nil {
		t.Fatal("failed connecting to database")
	}

	migrate(t, dbName)
}

func teardownTestDB(t *testing.T) {
	ctx := context.Background()
	ResetTelemetry()

	// Extract host:port and database name
	parts := strings.Split(dbProvider.connStr, "/")
	hostPortPart := parts[0]
	dbName := parts[1]
	dbName = strings.Split(dbName, "?")[0]

	// Connect to root using container's host:port
	rootConnStr := fmt.Sprintf("%s/?parseTime=true&multiStatements=true", hostPortPart)
	db, err := sql.Open("mysql", rootConnStr)
	if err != nil {
		t.Fatal(err)
	}
	defer db.Close()

	_, err = db.ExecContext(ctx, fmt.Sprintf("DROP DATABASE %s", dbName))
	if err != nil {
		t.Fatal(err)
	}
}

func migrate(t *testing.T, dbName string) {
	// Load SQL schema from file
	content, err := os.ReadFile("user.sql")
	if err != nil {
		t.Fatal(err)
	}

	db, err := dbProvider.GetDatabase("", false)
	if err != nil {
		t.Fatal(err)
	}

	_, err = db.ExecContext(context.Background(), string(content))
	if err != nil {
		t.Fatal(err)
	}
}

func TestUserCRUD(t *testing.T) {
	t.Run("CreateGetUpdateDelete", func(t *testing.T) {
		setupTestDB(t)
		defer teardownTestDB(t)

		// Initialize DAL
		userDAL := NewUserDAL(
			dbProvider,
			nil, // cache provider (mock if needed)
			nil, // config provider (mock if needed)
			gobreaker.Settings{},
		)

		ctx := context.Background()

		// Test Create
		newUser := &User{
			Age:       25,
			Email:     "test@example.com",
			Uuid:      uuid.New().String(),
			Status:    sql.NullString{String: "active", Valid: true},
			Birthdate: sql.NullTime{Time: time.Now(), Valid: true},
		}

		created, err := userDAL.Create(ctx, newUser)
		assert.NoError(t, err)
		assert.NotZero(t, created.ID)

		// Assert that the create operation telemetry counter has increased.
		createCounter := testutil.ToFloat64(dalOperationsTotalCounter.WithLabelValues("user", "create"))
		assert.Equal(t, 1.0, createCounter, "Expected one create operation")

		// --- Test GetByID ---
		fetched, err := userDAL.GetByID(ctx, created.ID)
		assert.NoError(t, err)
		assert.Equal(t, created.ID, fetched.ID)
		assert.Equal(t, "test@example.com", fetched.Email)

		getByIDCounter := testutil.ToFloat64(dalOperationsTotalCounter.WithLabelValues("user", "get_by_id"))
		assert.Equal(t, 1.0, getByIDCounter, "Expected one get_by_id operation")

		cacheMissesCounter := testutil.ToFloat64(dalCacheMissesCounter.WithLabelValues("user", "get_by_id"))
		cacheHitsCounter := testutil.ToFloat64(dalCacheHitsCounter.WithLabelValues("user", "get_by_id"))
		assert.Equal(t, 0.0, cacheMissesCounter, "Expected zero cache miss for get_by_id operation")
		assert.Equal(t, 1.0, cacheHitsCounter, "Expected one cache hits for get_by_id operation")

		// --- Test Update ---
		newEmail := "updated@example.com"
		fetched.Email = newEmail
		err = userDAL.Update(ctx, fetched)
		assert.NoError(t, err)

		updated, err := userDAL.GetByID(ctx, fetched.ID)
		assert.NoError(t, err)
		assert.Equal(t, newEmail, updated.Email)

		updateCounter := testutil.ToFloat64(dalOperationsTotalCounter.WithLabelValues("user", "update"))
		assert.Equal(t, 1.0, updateCounter, "Expected one update operation")

		cacheMissesCounter = testutil.ToFloat64(dalCacheMissesCounter.WithLabelValues("user", "get_by_id"))
		cacheHitsCounter = testutil.ToFloat64(dalCacheHitsCounter.WithLabelValues("user", "get_by_id"))
		assert.Equal(t, 0.0, cacheMissesCounter, "Expected zero cache miss for get_by_id operation")
		assert.Equal(t, 3.0, cacheHitsCounter, "Expected three cache hits for get_by_id operation")

		// --- Test Delete ---
		err = userDAL.Delete(ctx, updated)
		assert.NoError(t, err)

		// Depending on your implementation, the delete operation's label may vary.
		// Here we assume that Delete increments dalOperationsTotalCounter with label "user", "delete".
		deleteCounter := testutil.ToFloat64(dalOperationsTotalCounter.WithLabelValues("user", "delete"))
		assert.Equal(t, 1.0, deleteCounter, "Expected one delete operation")

		// Verify deletion
		_, err = userDAL.GetByID(ctx, updated.ID)
		assert.ErrorIs(t, err, ErrNotFound)
	})
}

func TestUserCreateBulk(t *testing.T) {
	t.Run("TestUserCreateBulk", func(t *testing.T) {
		setupTestDB(t)
		defer teardownTestDB(t)

		// Initialize DAL
		userDAL := NewUserDAL(
			dbProvider,
			nil, // cache provider (mock if needed)
			nil, // config provider (mock if needed)
			gobreaker.Settings{},
		)

		ctx := context.Background()

		const numEntries = 50
		var users []*User

		for i := 1; i <= numEntries; i++ {
			// Test Create
			newUser := &User{
				Age:       25,
				Email:     fmt.Sprintf("test_%02d@example.com", i),
				Uuid:      uuid.New().String(),
				Status:    sql.NullString{String: "active", Valid: true},
				Birthdate: sql.NullTime{Time: time.Now(), Valid: true},
			}
			users = append(users, newUser)
		}

		users, err := userDAL.CreateBulk(ctx, users)
		assert.NoError(t, err)
		assert.Equal(t, numEntries, len(users))

		// Assert that the create operation telemetry counter has increased.
		createCounter := testutil.ToFloat64(dalOperationsTotalCounter.WithLabelValues("user", "create_bulk"))
		assert.Equal(t, 1.0, createCounter, "Expected one create_bulk operation")

		// assert we got all the IDs
		for _, user := range users {
			assert.Greater(t, user.ID, int64(0))
		}

		// --- Test GetByID ---
		fetched, err := userDAL.GetByID(ctx, users[0].ID)
		assert.NoError(t, err)
		assert.Equal(t, users[0].ID, fetched.ID)
		assert.Equal(t, "test_01@example.com", fetched.Email)
	})
}

// DefaultConfigProvider is an implementation of ConfigProvider that
// always returns true for both BlockedReads and BlockedWrites.
type AlwaysBlockingConfigProvider struct{}

// BlockedReads always returns true.
func (d AlwaysBlockingConfigProvider) BlockedReads(entityName string) bool {
	return true
}

// BlockedWrites always returns true.
func (d AlwaysBlockingConfigProvider) BlockedWrites(entityName string) bool {
	return true
}

func TestUserBlockedReadsAndWrites(t *testing.T) {
	t.Run("TestUserBlockedReadsAndWrites", func(t *testing.T) {
		setupTestDB(t)
		defer teardownTestDB(t)

		configProvider := AlwaysBlockingConfigProvider{}
		// Initialize DAL
		userDAL := NewUserDAL(
			dbProvider,
			nil,            // cache provider (mock if needed)
			configProvider, // config provider (mock if needed)
			gobreaker.Settings{},
		)

		ctx := context.Background()

		// Test Create
		newUser := &User{
			Age:       25,
			Email:     "test@example.com",
			Uuid:      uuid.New().String(),
			Status:    sql.NullString{String: "active", Valid: true},
			Birthdate: sql.NullTime{Time: time.Now(), Valid: true},
		}

		_, err := userDAL.Create(ctx, newUser)
		assert.ErrorIs(t, err, ErrOperationBlocked)

		_, err = userDAL.GetByID(ctx, 1)
		assert.ErrorIs(t, err, ErrOperationBlocked)

		err = userDAL.Update(ctx, newUser)
		assert.ErrorIs(t, err, ErrOperationBlocked)

		err = userDAL.Delete(ctx, newUser)
		assert.ErrorIs(t, err, ErrOperationBlocked)

		_, err = userDAL.Store(ctx, newUser)
		assert.ErrorIs(t, err, ErrOperationBlocked)
	})
}

func TestUserGetEmail(t *testing.T) {
	t.Run("TestUserGetEmail", func(t *testing.T) {
		setupTestDB(t)
		defer teardownTestDB(t)

		// Initialize DAL
		userDAL := NewUserDAL(
			dbProvider,
			nil, // cache provider (mock if needed)
			nil, // config provider (mock if needed)
			gobreaker.Settings{},
		)

		ctx := context.Background()

		// Test Create
		newUser := &User{
			Age:       25,
			Email:     "test@example.com",
			Uuid:      uuid.New().String(),
			Status:    sql.NullString{String: "active", Valid: true},
			Birthdate: sql.NullTime{Time: time.Now(), Valid: true},
		}

		created, err := userDAL.Create(ctx, newUser)
		assert.NoError(t, err)
		assert.NotZero(t, created.ID)

		// Assert that the create operation telemetry counter has increased.
		createCounter := testutil.ToFloat64(dalOperationsTotalCounter.WithLabelValues("user", "create"))
		assert.Equal(t, 1.0, createCounter, "Expected one create operation")

		// --- Test GetByEmail
		fetched, err := userDAL.GetByEmail(ctx, "test@example.com")
		assert.NoError(t, err)
		assert.Equal(t, created.ID, fetched.ID)
		assert.Equal(t, "test@example.com", fetched.Email)

		getByEmailCounter := testutil.ToFloat64(dalOperationsTotalCounter.WithLabelValues("user", "get_by_email"))
		assert.Equal(t, 1.0, getByEmailCounter, "Expected one get_by_email operation")

		// no caching of get_by_id should happen here.
		cacheIDMissesCounter := testutil.ToFloat64(dalCacheMissesCounter.WithLabelValues("user", "get_by_id"))
		cacheIDHitsCounter := testutil.ToFloat64(dalCacheHitsCounter.WithLabelValues("user", "get_by_id"))
		assert.Equal(t, 0.0, cacheIDMissesCounter, "Expected zero cache miss for get_by_id operation")
		assert.Equal(t, 0.0, cacheIDHitsCounter, "Expected zero cache hits for get_by_id operation")

		cacheMissesCounter := testutil.ToFloat64(dalCacheMissesCounter.WithLabelValues("user", "get_by_email"))
		cacheHitsCounter := testutil.ToFloat64(dalCacheHitsCounter.WithLabelValues("user", "get_by_email"))
		assert.Equal(t, 1.0, cacheMissesCounter, "Expected one cache miss for get_by_email operation")
		assert.Equal(t, 0.0, cacheHitsCounter, "Expected zero cache hits for get_by_email operation")

		// Test missing entry
		_, err = userDAL.getByEmail(ctx, "nonexisting@example.com")
		assert.ErrorIs(t, err, ErrNotFound)

		// Test using cache
		fetched, err = userDAL.GetByEmail(ctx, "test@example.com")
		assert.NoError(t, err)
		assert.Equal(t, created.ID, fetched.ID)
		assert.Equal(t, "test@example.com", fetched.Email)

		getByEmailCounter = testutil.ToFloat64(dalOperationsTotalCounter.WithLabelValues("user", "get_by_email"))
		assert.Equal(t, 2.0, getByEmailCounter, "Expected two get_by_email operation")

		cacheMissesCounter = testutil.ToFloat64(dalCacheMissesCounter.WithLabelValues("user", "get_by_email"))
		cacheHitsCounter = testutil.ToFloat64(dalCacheHitsCounter.WithLabelValues("user", "get_by_email"))
		assert.Equal(t, 1.0, cacheMissesCounter, "Expected one cache miss for get_by_email operation")
		assert.Equal(t, 1.0, cacheHitsCounter, "Expected one cache hits for get_by_email operation")

		// caching now has id -> entity mapping so we should get one hit
		cacheIDMissesCounter = testutil.ToFloat64(dalCacheMissesCounter.WithLabelValues("user", "get_by_id"))
		cacheIDHitsCounter = testutil.ToFloat64(dalCacheHitsCounter.WithLabelValues("user", "get_by_id"))
		assert.Equal(t, 0.0, cacheIDMissesCounter, "Expected zero cache miss for get_by_id operation")
		assert.Equal(t, 1.0, cacheIDHitsCounter, "Expected one cache hits for get_by_id operation")
	})
}

func TestListById(t *testing.T) {
	t.Run("TestListById", func(t *testing.T) {
		setupTestDB(t)
		defer teardownTestDB(t)

		// Initialize DAL
		userDAL := NewUserDAL(
			dbProvider,
			nil, // cache provider (mock if needed)
			nil, // config provider (mock if needed)
			gobreaker.Settings{},
		)

		ctx := context.Background()

		const numEntries = 100
		// lets create some entries
		for i := 1; i <= numEntries; i++ {
			// Test Create
			newUser := &User{
				Age:       25,
				Email:     fmt.Sprintf("test_%d@example.com", i),
				Uuid:      uuid.New().String(),
				Status:    sql.NullString{String: "active", Valid: true},
				Birthdate: sql.NullTime{Time: time.Now(), Valid: true},
			}

			created, err := userDAL.Create(ctx, newUser)
			assert.NoError(t, err)
			assert.NotZero(t, created.ID)

			// Assert that the create operation telemetry counter has increased.
			createCounter := testutil.ToFloat64(dalOperationsTotalCounter.WithLabelValues("user", "create"))
			assert.Equal(t, float64(i), createCounter, "Expected enough create operations")
		}

		// test count
		count, err := userDAL.CountListById(ctx)
		assert.NoError(t, err)
		assert.Equal(t, int64(numEntries), count)

		// assert it is cached by running one more time.
		count, err = userDAL.CountListById(ctx)
		assert.NoError(t, err)
		assert.Equal(t, int64(numEntries), count)

		countListByIDCounter := testutil.ToFloat64(dalOperationsTotalCounter.WithLabelValues("user", "count_list_by_id"))
		assert.Equal(t, 2.0, countListByIDCounter, "Expected count_list_by_id counter to equal 2 after two calls")

		countCacheHitCounter := testutil.ToFloat64(dalCacheHitsCounter.WithLabelValues("user", "count_list_by_id"))
		assert.Equal(t, 1.0, countCacheHitCounter, "Expected one cache hit on count_list_by_id counter")

		// Test ListById with pagination.
		const pageSize = 10

		// First page: startID = 0 should return the first page.
		usersPage1, err := userDAL.ListById(ctx, 0, pageSize)
		assert.NoError(t, err)
		assert.Len(t, usersPage1, pageSize, "Expected first page to have exactly %d entries", pageSize)
		// Verify that the IDs are in ascending order.
		for i := 1; i < len(usersPage1); i++ {
			assert.True(t, usersPage1[i].ID > usersPage1[i-1].ID, "Expected IDs to be in ascending order")
		}

		// Next page: use the last ID from page1 as the starting point.
		lastID := usersPage1[len(usersPage1)-1].ID
		usersPage2, err := userDAL.ListById(ctx, lastID, pageSize)
		assert.NoError(t, err)
		// Expect page2 to have pageSize entries if there are enough users.
		assert.Len(t, usersPage2, pageSize, "Expected second page to have exactly %d entries", pageSize)
		// The first entry in page2 should have an ID greater than lastID.
		assert.True(t, usersPage2[0].ID > lastID, "Expected first user ID in page2 to be greater than last ID from page1")

		// Assert telemetry for ListById.
		// Two separate ListById calls should have incremented the counter by 2.
		listByIDCounter := testutil.ToFloat64(dalOperationsTotalCounter.WithLabelValues("user", "list_by_id"))
		assert.Equal(t, 2.0, listByIDCounter, "Expected list_by_id counter to equal 2 after two calls")

	})
}

func TestListByIdCachingAndInvalidation(t *testing.T) {
	t.Run("TestListByIdCachingAndInvalidation", func(t *testing.T) {
		setupTestDB(t)
		defer teardownTestDB(t)

		// Initialize the DAL with default providers (will use NoopCacheProvider and DefaultConfigProvider if nil).
		userDAL := NewUserDAL(
			dbProvider,
			nil, // cache provider (default if nil)
			nil, // config provider (default if nil)
			gobreaker.Settings{},
		)
		ctx := context.Background()
		const numEntries = 50
		const pageSize = 10

		// Insert a set of users.
		for i := 1; i <= numEntries; i++ {
			newUser := &User{
				Age:       25,
				Email:     fmt.Sprintf("cache_test_%d@example.com", i),
				Uuid:      uuid.New().String(),
				Status:    sql.NullString{String: "active", Valid: true},
				Birthdate: sql.NullTime{Time: time.Now(), Valid: true},
			}
			created, err := userDAL.Create(ctx, newUser)
			assert.NoError(t, err)
			assert.NotZero(t, created.ID)
		}

		// First call to ListById: this call should load data from the DB and set the cache.
		usersPage1, err := userDAL.ListById(ctx, 0, pageSize)
		assert.NoError(t, err)
		assert.Len(t, usersPage1, pageSize, "Expected first page to have %d entries", pageSize)

		// Record cache hit and miss counters after first call.
		hitsAfterFirst := testutil.ToFloat64(dalCacheHitsCounter.WithLabelValues("user", "list_by_id"))
		missesAfterFirst := testutil.ToFloat64(dalCacheMissesCounter.WithLabelValues("user", "list_by_id"))

		// Second call to ListById with the same parameters: should use the list cache and then individual item cache.
		usersPage1Cached, err := userDAL.ListById(ctx, 0, pageSize)
		assert.NoError(t, err)
		assert.Len(t, usersPage1Cached, pageSize, "Expected first page (cached) to have %d entries", pageSize)

		// Get updated telemetry counters.
		hitsAfterSecond := testutil.ToFloat64(dalCacheHitsCounter.WithLabelValues("user", "list_by_id"))
		missesAfterSecond := testutil.ToFloat64(dalCacheMissesCounter.WithLabelValues("user", "list_by_id"))

		// Expect that cache hits have increased on the second call.
		assert.Greater(t, hitsAfterSecond, hitsAfterFirst, "Expected cache hits to increase on second call")
		// Expect no new cache misses if items were found in cache.
		assert.Equal(t, missesAfterFirst, missesAfterSecond, "Expected cache misses to remain the same on second call")

		// Flush the list cache to simulate cache invalidation.
		userDAL.FlushListCache()

		// Third call to ListById: since list cache was flushed, the DAL should fall back to DB,
		// leading to new cache misses for the list retrieval.
		usersPage1AfterFlush, err := userDAL.ListById(ctx, 0, pageSize)
		assert.NoError(t, err)
		assert.Len(t, usersPage1AfterFlush, pageSize, "Expected first page to have %d entries after cache flush", pageSize)

		// After flush, cache misses should increase.
		missesAfterFlush := testutil.ToFloat64(dalCacheMissesCounter.WithLabelValues("user", "list_by_id"))
		assert.Greater(t, missesAfterFlush, missesAfterSecond, "Expected cache misses to increase after list cache flush")
	})
}

func TestListByBday(t *testing.T) {
	t.Run("TestListByBday", func(t *testing.T) {
		// Set up a fresh test database.
		setupTestDB(t)
		defer teardownTestDB(t)

		// Initialize the DAL with default cache and config providers.
		userDAL := NewUserDAL(
			dbProvider,
			nil, // cache provider (defaults to NoopCacheProvider if nil)
			nil, // config provider (defaults to DefaultConfigProvider if nil)
			gobreaker.Settings{},
		)
		ctx := context.Background()

		// Insert 12 users, one per month in 1990.
		for month := 1; month <= 12; month++ {
			birthdate := time.Date(1990, time.Month(month), 1, 0, 0, 0, 0, time.UTC)
			newUser := &User{
				Age:       30,
				Email:     fmt.Sprintf("user_%02d@example.com", month),
				Uuid:      uuid.New().String(),
				Status:    sql.NullString{String: "active", Valid: true},
				Birthdate: sql.NullTime{Time: birthdate, Valid: true},
			}
			created, err := userDAL.Create(ctx, newUser)
			assert.NoError(t, err)
			assert.NotZero(t, created.ID)
		}

		// Test ListByBday for March 1, 1990.
		bdayToTest := time.Date(1990, time.May, 1, 0, 0, 0, 0, time.UTC)
		nullBday := sql.NullTime{Time: bdayToTest, Valid: true}
		pageSize := 10

		count, err := userDAL.CountListByBday(ctx, nullBday)
		assert.NoError(t, err)
		assert.Equal(t, int64(4), count)

		// First call: should load from DB and populate the cache.
		results, err := userDAL.ListByBday(ctx, nullBday, 0, pageSize)
		assert.NoError(t, err)
		assert.Len(t, results, 4, "Expected 4 users for birthdate %v", bdayToTest)
		assert.Equal(t, "user_04@example.com", results[0].Email)

		// Second call: should hit the cache.
		resultsCached, err := userDAL.ListByBday(ctx, nullBday, 0, pageSize)
		assert.NoError(t, err)
		assert.Len(t, resultsCached, 4, "Expected 4 users from cache for birthdate %v", bdayToTest)
		assert.Equal(t, "user_04@example.com", resultsCached[0].Email)

		// Verify that the telemetry counter for "list_by_bday" has incremented by 2.
		listByBdayCounter := testutil.ToFloat64(dalOperationsTotalCounter.WithLabelValues("user", "list_by_bday"))
		assert.Equal(t, 2.0, listByBdayCounter, "Expected list_by_bday counter to equal 2 after two calls")
	})
}

func TestOptimisticLocking(t *testing.T) {
	t.Run("TestOptimisticLocking", func(t *testing.T) {
		// Set up a fresh test database.
		setupTestDB(t)
		defer teardownTestDB(t)

		// Initialize DAL with default providers.
		userDAL := NewUserDAL(
			dbProvider,
			nil, // default cache provider
			nil, // default config provider
			gobreaker.Settings{},
		)
		ctx := context.Background()

		// Create a user.
		newUser := &User{
			Age:       25,
			Email:     "optimistic@example.com",
			Uuid:      uuid.New().String(),
			Status:    sql.NullString{String: "active", Valid: true},
			Birthdate: sql.NullTime{Time: time.Now(), Valid: true},
		}
		created, err := userDAL.Create(ctx, newUser)
		assert.NoError(t, err)
		assert.NotZero(t, created.ID)

		// Simulate an external update that bumps the version.
		// We update the record directly so that the version in the database is incremented.
		db, err := dbProvider.GetDatabase("user", true)
		assert.NoError(t, err)
		_, err = db.ExecContext(ctx, "UPDATE users SET version = version + 1 WHERE id = ?", created.ID)
		assert.NoError(t, err)

		// Now try to update the user using the stale version information from 'created'.
		// Change a field.
		created.Email = "updated@example.com"

		// The update should use the stale version (e.g. X) while the DB has version (X+1), so no rows are affected.
		err = userDAL.Update(ctx, created)
		assert.ErrorIs(t, err, ErrNotFound, "Expected update to fail due to optimistic locking conflict")

		// new version should be higher than created
		retrieved, err := userDAL.GetByID(ctx, created.ID)
		assert.NoError(t, err)
		assert.Greater(t, retrieved.version, created.version)
	})
}
