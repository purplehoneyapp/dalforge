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
			"id":          {Type: "int64", AllowNull: false, Unique: true},
			"public_id":   {Type: "uid", Prefix: "user", AllowNull: false, Unique: true},
			"email":       {Type: "varchar", AllowNull: false, Unique: true},
			"preferences": {Type: "json", AllowNull: true, Unique: false}, // Injecting JSON column here
			"created":     {Type: "datetime", AllowNull: false, Unique: false},
			"updated":     {Type: "datetime", AllowNull: false, Unique: false},
		},
		Operations: OperationConfig{
			Gets: []string{"id", "email", "public_id"},
			Lists: []ListConfig{
				{
					Name:       "user_list",
					Where:      "email = :emailParam AND public_id = :pubId",
					Order:      "created",
					Descending: false,
					TypeMapping: map[string]string{
						"emailParam": "email",
						"pubId":      "public_id",
					},
				},
			},
			Deletes: []DeleteConfig{
				{
					Name:  "delete_expired",
					Where: "created < :cutoff",
					TypeMapping: map[string]string{
						"cutoff": "created",
					},
				},
			},
			Write:  true,
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
			"id":             {Type: "int64", AllowNull: false, Unique: true},
			"email":          {Type: "varchar", AllowNull: false, Unique: false},                   // error: used in gets but not unique
			"firstName":      {Type: "varchar", AllowNull: false, Unique: false},                   // error: not snake_case
			"invalid":        {Type: "unknown", AllowNull: false, Unique: false},                   // error: unsupported type
			"legacy_uuid":    {Type: "uuid", AllowNull: false, Unique: false},                      // error: uuid is no longer supported
			"missing_prefix": {Type: "uid", Prefix: "", AllowNull: false, Unique: true},            // error: uid requires a prefix when unique
			"bad_prefix":     {Type: "uid", Prefix: "User Prefix", AllowNull: false, Unique: true}, // error: prefix must be snake_case when unique
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
				{
					Name:       "invalid_mapping_list",
					Where:      "email = :badParam",
					Order:      "created",
					Descending: false,
					TypeMapping: map[string]string{
						"badParam": "ghost_column", // error: mapping to a column that doesn't exist
					},
				},
			},
			Deletes: []DeleteConfig{
				{
					Name: "del", // error: too short
				},
				{
					Name: "delete invalid mapping", // error: spaces, not snake_case
					TypeMapping: map[string]string{
						"badParam": "ghost_column", // error: ghost_column doesn't exist
					},
				},
			},
			Write:  true,
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
		"column 'legacy_uuid' has unsupported type 'uuid'",                    // Tests the removal of legacy uuid
		"column 'missing_prefix' of type 'uid' requires a non-empty 'prefix'", // Tests missing prefix (now triggers correctly)
		"prefix 'User Prefix' for column 'bad_prefix' must be in snake_case",  // Tests malformed prefix (now triggers correctly)
		"get operation refers to unknown column 'non_existent'",
		"get operation requires column 'email' to be unique",
		"list name 'lst' must be longer than 4 characters",
		"list 'lst' order must contain exactly one column",
		"order column 'unknown' in list 'long_list' is not defined and not a default column",
		"typeMapping column 'ghost_column' for param 'badParam' in list 'invalid_mapping_list' is not defined", // Tests invalid mapping
		"caching type must be 'redis'",
		"singleExpirationSeconds must be greater than 1",
		"listExpirationSeconds must be greater than 1",
		"listInvalidation must be 'flush' or 'expire'",
		"maxItemsCount must be greater than 10",
		"consecutiveFailures should be 1 or more",
		"timeoutSeconds should be 1 or more",
		"delete name 'del' must be longer than 4 characters",
		"delete name 'delete invalid mapping' must not contain spaces",
		"delete name 'delete invalid mapping' must be in snake_case",
		"typeMapping column 'ghost_column' for param 'badParam' in delete 'delete invalid mapping' is not defined",
	}

	for _, expectedError := range expectedErrors {
		if !strings.Contains(errStr, expectedError) {
			t.Errorf("Expected error message to contain: %q, got: %q", expectedError, errStr)
		}
	}
}
