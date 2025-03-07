package temp

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/sony/gobreaker"
)

var (
	ErrNotFound = errors.New("user not found")

	dalOperationsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dal_user_operations_total",
			Help: "Total number of user DAL operations",
		},
		[]string{"operation"},
	)

	dalOperationsErrors = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "dal_user_operations_errors_total",
			Help: "Total number of failed user DAL operations",
		},
		[]string{"operation"},
	)

	dalOperationsLatency = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "dal_user_operations_latency_seconds",
			Help:    "Latency distribution of user DAL operations",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"operation"},
	)
)

func init() {
	prometheus.MustRegister(dalOperationsTotal, dalOperationsErrors, dalOperationsLatency)
}

type User struct {
	ID        int64
	Email     string
	Created   time.Time
	Updated   time.Time
	Birthdate sql.NullTime
}

type UserDAL struct {
	db      *sql.DB
	breaker *gobreaker.CircuitBreaker
}

func NewUserDAL(db *sql.DB, settings gobreaker.Settings) *UserDAL {
	if settings.Name == "" {
		settings.Name = "user_dal"
	}
	if settings.ReadyToTrip == nil {
		settings.ReadyToTrip = func(counts gobreaker.Counts) bool {
			return counts.ConsecutiveFailures > 5
		}
	}
	if settings.Timeout == 0 {
		settings.Timeout = time.Second * 30
	}

	return &UserDAL{
		db:      db,
		breaker: gobreaker.NewCircuitBreaker(settings),
	}
}
func (d *UserDAL) GetByID(ctx context.Context, id int64) (*User, error) {
	const operation = "get_by_id"
	start := time.Now()
	dalOperationsTotal.WithLabelValues(operation).Inc()

	result, err := d.breaker.Execute(func() (interface{}, error) {
		return d.getByID(ctx, id)
	})

	if err != nil {
		dalOperationsErrors.WithLabelValues(operation).Inc()
	}

	dalOperationsLatency.WithLabelValues(operation).Observe(time.Since(start).Seconds())

	if result == nil {
		return nil, err
	}
	return result.(*User), err
}

func (d *UserDAL) getByID(ctx context.Context, id int64) (*User, error) {
	query := `
		SELECT id, email, created, updated, birthdate
		FROM users
		WHERE id = ?
	`
	row := d.db.QueryRowContext(ctx, query, id)
	var user User
	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.Created,
		&user.Updated,
		&user.Birthdate,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get user by ID: %w", err)
	}
	return &user, nil
}

func (d *UserDAL) GetByEmail(ctx context.Context, email string) (*User, error) {
	const operation = "get_by_email"
	start := time.Now()
	dalOperationsTotal.WithLabelValues(operation).Inc()

	result, err := d.breaker.Execute(func() (interface{}, error) {
		return d.getByEmail(ctx, email)
	})

	if err != nil {
		dalOperationsErrors.WithLabelValues(operation).Inc()
	}

	dalOperationsLatency.WithLabelValues(operation).Observe(time.Since(start).Seconds())

	if result == nil {
		return nil, err
	}
	return result.(*User), err
}

func (d *UserDAL) getByEmail(ctx context.Context, email string) (*User, error) {
	query := `
		SELECT id, email, created, updated, birthdate
		FROM users
		WHERE email = ?
	`
	row := d.db.QueryRowContext(ctx, query, email)
	var user User
	err := row.Scan(
		&user.ID,
		&user.Email,
		&user.Created,
		&user.Updated,
		&user.Birthdate,
	)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, ErrNotFound
		}
		return nil, fmt.Errorf("failed to get user by email: %w", err)
	}
	return &user, nil
}

func (d *UserDAL) ListById(ctx context.Context, startID int64, pageSize int) ([]*User, error) {
	const operation = "list_by_id"
	start := time.Now()
	dalOperationsTotal.WithLabelValues(operation).Inc()

	result, err := d.breaker.Execute(func() (interface{}, error) {
		return d.listById(ctx, startID, pageSize)
	})

	if err != nil {
		dalOperationsErrors.WithLabelValues(operation).Inc()
	}

	dalOperationsLatency.WithLabelValues(operation).Observe(time.Since(start).Seconds())

	if result == nil {
		return nil, err
	}
	return result.([]*User), err
}

func (d *UserDAL) listById(ctx context.Context, startID int64, pageSize int) ([]*User, error) {
	query := `
		SELECT id, email, created, updated, birthdate
		FROM users
		WHERE id >= ?
		ORDER BY id ASC
		LIMIT ?
	`
	rows, err := d.db.QueryContext(ctx, query, startID, pageSize)
	if err != nil {
		return nil, fmt.Errorf("failed to query users: %w", err)
	}
	defer rows.Close()

	var users []*User
	for rows.Next() {
		var user User
		err := rows.Scan(
			&user.ID,
			&user.Email,
			&user.Created,
			&user.Updated,
			&user.Birthdate,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan user: %w", err)
		}
		users = append(users, &user)
	}

	if err = rows.Err(); err != nil {
		return nil, fmt.Errorf("rows error: %w", err)
	}

	return users, nil
}

func (d *UserDAL) Delete(ctx context.Context, id int64) error {
	const operation = "delete"
	start := time.Now()
	dalOperationsTotal.WithLabelValues(operation).Inc()

	_, err := d.breaker.Execute(func() (interface{}, error) {
		return nil, d.delete(ctx, id)
	})

	if err != nil {
		dalOperationsErrors.WithLabelValues(operation).Inc()
	}

	dalOperationsLatency.WithLabelValues(operation).Observe(time.Since(start).Seconds())
	return err
}

func (d *UserDAL) delete(ctx context.Context, id int64) error {
	query := `
		DELETE FROM users
		WHERE id = ?
	`
	res, err := d.db.ExecContext(ctx, query, id)
	if err != nil {
		return fmt.Errorf("failed to delete user: %w", err)
	}

	rowsAffected, err := res.RowsAffected()
	if err != nil {
		return fmt.Errorf("failed to get rows affected: %w", err)
	}

	if rowsAffected == 0 {
		return ErrNotFound
	}

	return nil
}

func (d *UserDAL) Store(ctx context.Context, user *User) (*User, error) {
	const operation = "store"
	start := time.Now()
	dalOperationsTotal.WithLabelValues(operation).Inc()

	result, err := d.breaker.Execute(func() (interface{}, error) {
		return d.store(ctx, user)
	})

	if err != nil {
		dalOperationsErrors.WithLabelValues(operation).Inc()
	}

	dalOperationsLatency.WithLabelValues(operation).Observe(time.Since(start).Seconds())

	if result == nil {
		return nil, err
	}
	return result.(*User), err
}

func (d *UserDAL) store(ctx context.Context, user *User) (*User, error) {
	query := `
		INSERT INTO users (email, birthdate)
		VALUES (?, ?)
	`
	result, err := d.db.ExecContext(ctx, query, user.Email, user.Birthdate)
	if err != nil {
		return nil, fmt.Errorf("failed to insert user: %w", err)
	}

	id, err := result.LastInsertId()
	if err != nil {
		return nil, fmt.Errorf("failed to get last insert ID: %w", err)
	}

	user.ID = id
	return user, nil
}
