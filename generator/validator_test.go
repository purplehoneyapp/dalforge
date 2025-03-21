package generator

import (
	"strings"
	"testing"
)

func TestValidateEntityConfig_Valid(t *testing.T) {
	validConfig := EntityConfig{
		Name:    "user", // snake_case, >3 chars
		Version: "v1",   // non-empty and valid format
		Columns: map[string]Column{
			"id":      {Type: "int64", AllowNull: false, Unique: true},
			"email":   {Type: "string", AllowNull: false, Unique: true},
			"created": {Type: "datetime", AllowNull: false, Unique: false},
			"updated": {Type: "datetime", AllowNull: false, Unique: false},
		},
		Operations: OperationConfig{
			Gets: []string{"id", "email"},
			Lists: []ListConfig{
				{
					Name:       "user_list",
					Where:      "email = 'a@example.com'",
					Order:      "created",
					Descending: false,
				},
			},
			Store:  true,
			Delete: true,
		},
		CircuitBreaker: CircuitBreakerConfig{
			TimeoutSeconds:      30,
			ConsecutiveFailures: 5,
		},
		Caching: CachingConfig{
			Type:                    "redis",
			SingleExpirationSeconds: 60,
			ListExpirationSeconds:   60,
			ListInvalidation:        "flush",
			MaxItemsCount:           20,
		},
	}

	err := ValidateEntityConfig(validConfig)
	if err != nil {
		t.Errorf("Expected valid config to pass validation, got error: %v", err)
	}
}

func TestValidateEntityConfig_Invalid(t *testing.T) {
	invalidConfig := EntityConfig{
		Name:    "User", // not snake_case
		Version: "",     // empty version (error)
		Columns: map[string]Column{
			"id":        {Type: "int64", AllowNull: false, Unique: true},
			"email":     {Type: "string", AllowNull: false, Unique: false},  // error: used in gets but not unique
			"firstName": {Type: "string", AllowNull: false, Unique: false},  // error: not snake_case
			"invalid":   {Type: "unknown", AllowNull: false, Unique: false}, // error: unsupported type
		},
		Operations: OperationConfig{
			Gets: []string{"id", "email", "non_existent"}, // non_existent: error; email: error due to not unique.
			Lists: []ListConfig{
				{
					Name:       "lst", // too short (must be > 4 chars)
					Where:      "email = 'a@example.com'",
					Order:      "email,created", // error: order must contain exactly one column
					Descending: false,
				},
				{
					Name:       "long_list",
					Where:      "",
					Order:      "unknown", // error: order column not defined in columns and not a default
					Descending: true,
				},
			},
			Store:  true,
			Delete: true,
		},
		CircuitBreaker: CircuitBreakerConfig{
			TimeoutSeconds:      0,
			ConsecutiveFailures: 0,
		},
		Caching: CachingConfig{
			Type:                    "memory",  // error: must be "redis"
			SingleExpirationSeconds: 1,         // error: must be > 1
			ListExpirationSeconds:   0,         // error: must be > 1
			ListInvalidation:        "invalid", // error: must be "flush" or "expire"
			MaxItemsCount:           5,         // error: must be > 10
		},
	}

	err := ValidateEntityConfig(invalidConfig)
	if err == nil {
		t.Error("Expected invalid config to produce errors, got nil")
	}
	errStr := err.Error()

	expectedErrors := []string{
		"entity name must be in snake_case",
		"version cannot be empty",
		"column name 'firstName' must be in snake_case",
		"column 'invalid' has unsupported type 'unknown'",
		"get operation refers to unknown column 'non_existent'",
		"get operation requires column 'email' to be unique",
		"list name 'lst' must be longer than 4 characters",
		"list 'lst' order must contain exactly one column",
		"order column 'unknown' in list 'long_list' is not defined and not a default column",
		"caching type must be 'redis'",
		"singleExpirationSeconds must be greater than 1",
		"listExpirationSeconds must be greater than 1",
		"listInvalidation must be 'flush' or 'expire'",
		"maxItemsCount must be greater than 10",
		"consecutiveFailures should be 1 or more",
		"timeoutSeconds should be 1 or more",
	}

	for _, expectedError := range expectedErrors {
		if !strings.Contains(errStr, expectedError) {
			t.Errorf("Expected error message to contain: %q, got: %q", expectedError, errStr)
		}
	}
}
