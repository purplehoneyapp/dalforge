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

		const numEntries = 1000
		// lets create 1000 entries
		for i := 1; i < numEntries; i++ {
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
	})
}
