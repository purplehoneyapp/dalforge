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
	if len(name) < 3 {
		errs = append(errs, "entity name must be at least 3 characters")
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
		"varchar":  true,
		"text":     true,
		"bool":     true,
		"date":     true,
		"time":     true,
		"datetime": true,
		"uid":      true,
		"json":     true,
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

		// New validation rule for uid prefixes
		if col.Type == "uid" && col.Unique {
			if strings.TrimSpace(col.Prefix) == "" {
				errs = append(errs, fmt.Sprintf("column '%s' of type 'uid' requires a non-empty 'prefix'", colName))
			} else if !isSnakeCase(col.Prefix) {
				errs = append(errs, fmt.Sprintf("prefix '%s' for column '%s' must be in snake_case", col.Prefix, colName))
			}
		}
	}
	return errs
}

// validateOperationConfig validates Gets and Lists in operations.
// For Gets: each referenced column must exist and be marked as unique.
// For Lists: it calls validateListConfigs.
// validateOperationConfig validates Gets, Lists, and Deletes in operations.
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

	// Validate GetsBulk (NEW)
	errs = append(errs, validateGetsBulk(ops.GetsBulk, columns)...)

	// Validate Lists.
	errs = append(errs, validateListConfigs(ops.Lists, columns)...)

	// Validate Deletes.
	errs = append(errs, validateDeleteConfigs(ops.Deletes, columns)...)

	// Validate UpdatesBulk (NEW)
	errs = append(errs, validateUpdatesBulk(ops.UpdatesBulk, columns)...)

	return errs
}

// validateGetsBulk ensures that columns used for bulk gets exist and are unique.
func validateGetsBulk(getsBulk []string, columns map[string]Column) []string {
	var errs []string
	for _, colName := range getsBulk {
		if colName == "id" {
			continue // ID is perfectly fine to use for bulk gets
		}
		col, exists := columns[colName]
		if !exists {
			errs = append(errs, fmt.Sprintf("getsBulk operation refers to unknown column '%s'", colName))
		} else if !col.Unique {
			errs = append(errs, fmt.Sprintf("getsBulk operation requires column '%s' to be unique", colName))
		}
	}
	return errs
}

// validateUpdatesBulk ensures bulk updates are configured correctly safely.
func validateUpdatesBulk(updates []UpdateBulkConfig, columns map[string]Column) []string {
	var errs []string

	for _, upd := range updates {
		// 1. Validate Name
		if len(upd.Name) <= 4 {
			errs = append(errs, fmt.Sprintf("updatesBulk name '%s' must be longer than 4 characters", upd.Name))
		}
		if strings.Contains(upd.Name, " ") {
			errs = append(errs, fmt.Sprintf("updatesBulk name '%s' must not contain spaces", upd.Name))
		}
		if !isSnakeCase(upd.Name) {
			errs = append(errs, fmt.Sprintf("updatesBulk name '%s' must be in snake_case", upd.Name))
		}

		// 2. Validate WhereIn column
		if upd.WhereIn != "id" {
			if _, exists := columns[upd.WhereIn]; !exists {
				errs = append(errs, fmt.Sprintf("updatesBulk '%s' refers to unknown whereIn column '%s'", upd.Name, upd.WhereIn))
			}
		}

		// 3. Validate Set columns
		if len(upd.Set) == 0 {
			errs = append(errs, fmt.Sprintf("updatesBulk '%s' must specify at least one column in 'set'", upd.Name))
		}
		for _, setCol := range upd.Set {
			if setCol == "id" || setCol == "created" || setCol == "updated" {
				errs = append(errs, fmt.Sprintf("updatesBulk '%s' cannot update reserved column '%s'", upd.Name, setCol))
			} else if _, exists := columns[setCol]; !exists {
				errs = append(errs, fmt.Sprintf("updatesBulk '%s' refers to unknown set column '%s'", upd.Name, setCol))
			}
		}
	}
	return errs
}

// validateDeleteConfigs validates each delete config.
func validateDeleteConfigs(deletes []DeleteConfig, columns map[string]Column) []string {
	var errs []string
	allowedDefaults := map[string]bool{"created": true, "id": true, "updated": true}

	for _, del := range deletes {
		if len(del.Name) <= 4 {
			errs = append(errs, fmt.Sprintf("delete name '%s' must be longer than 4 characters", del.Name))
		}
		if strings.Contains(del.Name, " ") {
			errs = append(errs, fmt.Sprintf("delete name '%s' must not contain spaces", del.Name))
		}
		if !isSnakeCase(del.Name) {
			errs = append(errs, fmt.Sprintf("delete name '%s' must be in snake_case. eg. delete_expired", del.Name))
		}

		// Validate TypeMapping targets exist
		for paramName, mappedCol := range del.TypeMapping {
			if _, exists := columns[mappedCol]; !exists && !allowedDefaults[mappedCol] {
				errs = append(errs, fmt.Sprintf("typeMapping column '%s' for param '%s' in delete '%s' is not defined", mappedCol, paramName, del.Name))
			}
		}
	}
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

		// Validate TypeMapping targets exist
		for paramName, mappedCol := range list.TypeMapping {
			if _, exists := columns[mappedCol]; !exists && !allowedDefaults[mappedCol] {
				errs = append(errs, fmt.Sprintf("typeMapping column '%s' for param '%s' in list '%s' is not defined", mappedCol, paramName, list.Name))
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
