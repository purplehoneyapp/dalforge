package dal

import (
	"context"
	"database/sql"
	"log"
	"os"
	"strings"
	"testing"
	"time"

	_ "github.com/go-sql-driver/mysql" // MySQL driver
	"github.com/sony/gobreaker"
	"github.com/stretchr/testify/assert"
)

// Database connection details
const dsn = "root:root@tcp(127.0.0.1:3306)/testdb?parseTime=true"

// Connect to MySQL
func connectTestDB() *sql.DB {
	db, err := sql.Open("mysql", dsn)
	if err != nil {
		log.Fatalf("Failed to connect to MySQL: %v", err)
	}

	// Ensure connection is alive
	if err := db.Ping(); err != nil {
		log.Fatalf("Database not reachable: %v", err)
	}
	return db
}

// Read and execute schema.sql
func setupTestDB(db *sql.DB) {
	// Step 1: Drop the existing table
	_, err := db.Exec("DROP TABLE IF EXISTS users;")
	if err != nil {
		log.Fatalf("Failed to drop table: %v", err)
	}

	// Step 2: Read and execute schema.sql
	schemaFile := "dalexample.sql"
	sqlBytes, err := os.ReadFile(schemaFile)
	if err != nil {
		log.Fatalf("Failed to read schema file: %v", err)
	}

	// Step 2: Split SQL into individual statements
	sqlStatements := strings.Split(string(sqlBytes), ";") // Split at semicolons

	// Step 3: Execute each statement separately
	for _, stmt := range sqlStatements {
		stmt = strings.TrimSpace(stmt) // Remove spaces/newlines
		if stmt == "" {
			continue // Skip empty statements
		}
		_, err := db.Exec(stmt)
		if err != nil {
			log.Fatalf("Failed to execute statement: %s\nError: %v", stmt, err)
		}
	}

	// Step 4: Insert test data
	_, err = db.Exec(`
		INSERT INTO users (age, birthdate, email) VALUES
		(30, '1993-05-12', 'alice@example.com'),
		(28, '1995-02-20', 'bob@example.com');
	`)
	if err != nil {
		log.Fatalf("Failed to insert test data: %v", err)
	}
}

// Test GetByID
func TestGetByID(t *testing.T) {
	db := connectTestDB()
	defer db.Close()
	setupTestDB(db)

	userDAL := NewUserDAL(db, gobreaker.Settings{})

	ctx := context.Background()
	user, err := userDAL.GetByID(ctx, 1)

	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, int64(1), user.ID)
	assert.Equal(t, "alice@example.com", user.Email)
}

// Test GetByEmail
func TestGetByEmail(t *testing.T) {
	db := connectTestDB()
	defer db.Close()
	setupTestDB(db)

	userDAL := NewUserDAL(db, gobreaker.Settings{})

	ctx := context.Background()
	user, err := userDAL.GetByEmail(ctx, "bob@example.com")

	assert.NoError(t, err)
	assert.NotNil(t, user)
	assert.Equal(t, "bob@example.com", user.Email)
}

// Test Store
func TestStoreUser(t *testing.T) {
	db := connectTestDB()
	defer db.Close()
	setupTestDB(db)

	userDAL := NewUserDAL(db, gobreaker.Settings{})

	newUser := User{
		Age:       25,
		Birthdate: sql.NullTime{Time: time.Now(), Valid: true},
		Email:     "newuser@example.com",
		Created:   time.Now(),
		Updated:   time.Now(),
	}

	ctx := context.Background()
	storedUser, err := userDAL.Store(ctx, &newUser)

	assert.NoError(t, err)
	assert.NotNil(t, storedUser)
	assert.Greater(t, storedUser.ID, int64(2)) // ID should be >2 since 2 test users exist
}

// Test Delete
func TestDeleteUser(t *testing.T) {
	db := connectTestDB()
	defer db.Close()
	setupTestDB(db)

	userDAL := NewUserDAL(db, gobreaker.Settings{})

	ctx := context.Background()
	err := userDAL.Delete(ctx, 1) // Delete Alice

	assert.NoError(t, err)

	// Verify deletion
	user, err := userDAL.GetByID(ctx, 1)
	assert.Error(t, err) // Should return an error (not found)
	assert.Nil(t, user)
}

// Test ListById
func TestListById(t *testing.T) {
	db := connectTestDB()
	defer db.Close()
	setupTestDB(db)

	userDAL := NewUserDAL(db, gobreaker.Settings{})

	ctx := context.Background()
	users, err := userDAL.ListById(ctx, 1, 2)

	assert.NoError(t, err)
	assert.Equal(t, 2, len(users)) // Should return 2 users
}

// Test DeleteByEmail
func TestDeleteByEmail(t *testing.T) {
	db := connectTestDB()
	defer db.Close()
	setupTestDB(db)

	userDAL := NewUserDAL(db, gobreaker.Settings{})

	ctx := context.Background()
	err := userDAL.DeleteByEmail(ctx, "bob@example.com")

	assert.NoError(t, err)

	// Verify deletion
	user, err := userDAL.GetByEmail(ctx, "bob@example.com")
	assert.Error(t, err) // Should return an error (not found)
	assert.Nil(t, user)
}
