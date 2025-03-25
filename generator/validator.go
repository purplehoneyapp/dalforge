package generator

import (
	"errors"
	"fmt"
	"regexp"
	"strings"
)

// Main validation function that calls smaller validators.
func ValidateEntityConfig(entity EntityConfig) error {
	var errs []string

	// Validate entity name.
	errs = append(errs, validateEntityName(entity.Name)...)

	// Validate version.
	errs = append(errs, validateVersion(entity.Version)...)

	// Validate columns.
	errs = append(errs, validateColumns(entity.Columns)...)

	// Validate operations.
	errs = append(errs, validateOperationConfig(entity.Operations, entity.Columns)...)

	// Validate caching config.
	errs = append(errs, validateCachingConfig(entity.Caching)...)

	// Validate circuitbreaker config.
	errs = append(errs, validateCircuitBreakerConfig(entity.CircuitBreaker)...)

	if len(errs) > 0 {
		return errors.New(strings.Join(errs, "\n"))
	}
	return nil
}

// validateEntityName verifies the entity name is longer than 3 characters,
// has no spaces and is snake_cased.
func validateEntityName(name string) []string {
	var errs []string
	if len(name) <= 3 {
		errs = append(errs, "entity name must be longer than 3 characters")
	}
	if strings.Contains(name, " ") {
		errs = append(errs, "entity name must not contain spaces")
	}
	if !isSnakeCase(name) {
		errs = append(errs, "entity name must be in snake_case")
	}
	return errs
}

// validateVersion ensures version is not empty and follows a pattern like "v1", "v2", etc.
func validateVersion(version string) []string {
	var errs []string
	if strings.TrimSpace(version) == "" {
		errs = append(errs, "version cannot be empty")
	} else {
		if match, _ := regexp.MatchString(`^v\d+$`, version); !match {
			errs = append(errs, "version should be in the format v<number>, e.g., v1, v2")
		}
	}
	return errs
}

// validateColumns checks that each column name is snake_cased, has no spaces,
// and the type is supported.
func validateColumns(columns map[string]Column) []string {
	var errs []string
	allowedTypes := map[string]bool{
		"int8":     true,
		"int16":    true,
		"int32":    true,
		"int64":    true,
		"float":    true,
		"string":   true,
		"bool":     true,
		"date":     true,
		"time":     true,
		"datetime": true,
		"uuid":     true,
	}

	for colName, col := range columns {
		if strings.Contains(colName, " ") {
			errs = append(errs, fmt.Sprintf("column name '%s' must not contain spaces", colName))
		}
		if !isSnakeCase(colName) {
			errs = append(errs, fmt.Sprintf("column name '%s' must be in snake_case", colName))
		}
		if !allowedTypes[col.Type] {
			errs = append(errs, fmt.Sprintf("column '%s' has unsupported type '%s'", colName, col.Type))
		}
	}
	return errs
}

// validateOperationConfig validates Gets and Lists in operations.
// For Gets: each referenced column must exist and be marked as unique.
// For Lists: it calls validateListConfigs.
func validateOperationConfig(ops OperationConfig, columns map[string]Column) []string {
	var errs []string
	// Validate Gets.
	for _, colName := range ops.Gets {
		col, exists := columns[colName]
		if !exists {
			errs = append(errs, fmt.Sprintf("get operation refers to unknown column '%s'", colName))
		} else if !col.Unique {
			errs = append(errs, fmt.Sprintf("get operation requires column '%s' to be unique", colName))
		}
	}
	// Validate Lists.
	errs = append(errs, validateListConfigs(ops.Lists, columns)...)
	return errs
}

// validateListConfigs validates each list config:
//   - List name must be longer than 4 characters, snake_cased, and have no spaces.
//   - Order: if provided, must contain exactly one column that exists in the columns map
//     or is one of the allowed defaults: "created", "id", or "updated".
func validateListConfigs(lists []ListConfig, columns map[string]Column) []string {
	var errs []string
	allowedDefaults := map[string]bool{"created": true, "id": true, "updated": true}

	for _, list := range lists {
		if len(list.Name) <= 4 {
			errs = append(errs, fmt.Sprintf("list name '%s' must be longer than 4 characters", list.Name))
		}
		if strings.Contains(list.Name, " ") {
			errs = append(errs, fmt.Sprintf("list name '%s' must not contain spaces", list.Name))
		}
		if !isSnakeCase(list.Name) {
			errs = append(errs, fmt.Sprintf("list name '%s' must be in snake_case. eg. list_by_age, most_recent", list.Name))
		}

		order := strings.TrimSpace(list.Order)
		if order != "" {
			parts := strings.Split(order, ",")
			if len(parts) != 1 {
				errs = append(errs, fmt.Sprintf("list '%s' order must contain exactly one column", list.Name))
			} else {
				orderCol := strings.TrimSpace(parts[0])
				if strings.Contains(orderCol, " ") {
					errs = append(errs, fmt.Sprintf("order column '%s' in list '%s' must not contain spaces", orderCol, list.Name))
				}
				if _, exists := columns[orderCol]; !exists && !allowedDefaults[orderCol] {
					errs = append(errs, fmt.Sprintf("order column '%s' in list '%s' is not defined and not a default column", orderCol, list.Name))
				}
			}
		}
	}
	return errs
}

// validateCachingConfig validates caching configuration.
func validateCachingConfig(c CachingConfig) []string {
	var errs []string
	if strings.ToLower(c.Type) != "redis" {
		errs = append(errs, fmt.Sprintf("caching type must be 'redis', got '%s'", c.Type))
	}
	if c.SingleExpirationSeconds <= 1 {
		errs = append(errs, "singleExpirationSeconds must be greater than 1")
	}
	if c.ListExpirationSeconds <= 1 {
		errs = append(errs, "listExpirationSeconds must be greater than 1")
	}
	if c.ListInvalidation != "flush" && c.ListInvalidation != "expire" {
		errs = append(errs, fmt.Sprintf("listInvalidation must be 'flush' or 'expire', got '%s'", c.ListInvalidation))
	}
	if c.MaxItemsCount <= 10 {
		errs = append(errs, "maxItemsCount must be greater than 10")
	}
	return errs
}

func validateCircuitBreakerConfig(c CircuitBreakerConfig) []string {
	var errs []string
	if c.ConsecutiveFailures < 1 {
		errs = append(errs, fmt.Sprintf("consecutiveFailures should be 1 or more. got '%v'", c.ConsecutiveFailures))
	}
	if c.TimeoutSeconds < 1 {
		errs = append(errs, fmt.Sprintf("timeoutSeconds should be 1 or more. got '%v'", c.TimeoutSeconds))
	}
	return errs
}

// isSnakeCase checks if a string is in snake_case: only lowercase letters, numbers, and underscores.
func isSnakeCase(s string) bool {
	re := regexp.MustCompile(`^[a-z0-9]+(_[a-z0-9]+)*$`)
	return re.MatchString(s)
}
