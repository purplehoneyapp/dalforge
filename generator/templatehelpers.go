package generator

import (
	"fmt"
	"strings"
	"unicode"

	"golang.org/x/text/cases"
	"golang.org/x/text/language"
)

// Add these helper functions
var (
	titleCaser = cases.Title(language.English, cases.NoLower)
	lowerCaser = cases.Lower(language.English)
)

// PascalCaser converts a string to PascalCase
func PascalCaser(s string) string {
	parts := splitIntoParts(s)
	for i, p := range parts {
		parts[i] = titleCaser.String(lowerCaser.String(p))
	}
	return strings.Join(parts, "")
}

// CamelCaser converts a string to camelCase
func CamelCaser(s string) string {
	parts := splitIntoParts(s)
	for i, p := range parts {
		if i == 0 {
			// First word is lowercase
			parts[i] = lowerCaser.String(p)
		} else {
			// Subsequent words are title-cased
			parts[i] = titleCaser.String(lowerCaser.String(p))
		}
	}
	return strings.Join(parts, "")
}

// SnakeCaser converts a string to snake_case
func SnakeCaser(s string) string {
	var result []rune
	for i, r := range s {
		if unicode.IsUpper(r) {
			// Add underscore before uppercase letters (except the first character)
			if i > 0 {
				result = append(result, '_')
			}
			result = append(result, unicode.ToLower(r))
		} else {
			result = append(result, r)
		}
	}
	return string(result)
}

// Helper function to split a string into parts
func splitIntoParts(s string) []string {
	return strings.FieldsFunc(s, func(r rune) bool {
		return r == '_' || r == ' ' || r == '-'
	})
}

func toColumnName(s string) string {
	return lowerCaser.String(s)
}

func toGoType(yamlType string, allowNull bool) string {
	switch yamlType {
	case "int8":
		if allowNull {
			return "sql.NullInt16" // int8 maps to NullInt16 in Go
		}
		return "int8"
	case "int32":
		if allowNull {
			return "sql.NullInt32"
		}
		return "int32"
	case "int64":
		if allowNull {
			return "sql.NullInt64"
		}
		return "int64"
	case "float":
		if allowNull {
			return "sql.NullFloat64"
		}
		return "float64"
	case "uuid":
		if allowNull {
			return "sql.NullString"
		}
		return "string"
	case "string":
		if allowNull {
			return "sql.NullString"
		}
		return "string"
	case "bool":
		if allowNull {
			return "sql.NullBool"
		}
		return "bool"
	case "date", "time", "datetime":
		if allowNull {
			return "sql.NullTime"
		}
		return "time.Time"
	default:
		return "interface{}" // Fallback for unknown types
	}
}

func toSQLType(yamlType string) string {
	switch yamlType {
	case "int8":
		return "TINYINT"
	case "int32":
		return "INT"
	case "int64":
		return "BIGINT"
	case "float":
		return "DOUBLE"
	case "string":
		return "VARCHAR(255)" // Default length for strings
	case "bool":
		return "BOOLEAN"
	case "date":
		return "DATE"
	case "time":
		return "TIME"
	case "datetime":
		return "DATETIME"
	case "uuid":
		return "CHAR(36)"
	default:
		return "TEXT" // Fallback for unknown types
	}
}

func dict(values ...interface{}) map[string]interface{} {
	if len(values)%2 != 0 {
		panic("dict must have even number of arguments")
	}
	m := make(map[string]interface{})
	for i := 0; i < len(values); i += 2 {
		key, ok := values[i].(string)
		if !ok {
			panic("dict keys must be strings")
		}
		m[key] = values[i+1]
	}
	return m
}

func join(sep string, items ...string) string {
	return strings.Join(items, sep)
}

func keys(m interface{}) []string {
	var result []string

	switch v := m.(type) {
	case map[string]Column: // ✅ Handle map[string]Column
		for key := range v {
			result = append(result, key)
		}
	case map[string]interface{}:
		for key := range v {
			result = append(result, key)
		}
	default:
		panic(fmt.Sprintf("keys function expects a map[string]Column or map[string]interface{}, got %T", m))
	}

	return result
}

func sub(a, b int) int {
	return a - b
}

func add(a, b int) int {
	return a + b
}

// Gets all the column names from all lists config and output all indexes.
func listSQLIndexes(tableName string, columns map[string]Column, lists []ListConfig) string {
	// Create a set to keep track of columns that need an index.
	indexedColumns := make(map[string]bool)
	defaultColumns := map[string]bool{
		"created": true,
		"updated": true,
	}

	// Iterate over each list configuration.
	for _, list := range lists {
		for colName := range columns {
			// Check if the column name is found in the Where or Order clause.
			// This uses a simple substring match. You might want to refine this
			// with more precise parsing if needed.
			if strings.Contains(list.Where, colName) || strings.Contains(list.Order, colName) {
				indexedColumns[colName] = true
			}
		}
		for colName := range defaultColumns {
			if strings.Contains(list.Where, colName) || strings.Contains(list.Order, colName) {
				indexedColumns[colName] = true
			}
		}
	}

	// Build the CREATE INDEX statements.
	var builder strings.Builder
	for colName := range indexedColumns {
		// Customize the table name as needed.
		builder.WriteString(fmt.Sprintf("CREATE INDEX idx_%s ON %ss (%s);\n",
			SnakeCaser(colName), SnakeCaser(tableName), SnakeCaser(colName)))
	}

	return builder.String()
}

/*
	 Would output something like this for any column that has get operation:
		var oldEmail string
		if existing.Email != entity.Email {
			oldEmail = existing.Email
		}
*/
func checkColumnsChanged(config EntityConfig) string {
	result := ""
	for _, colName := range config.Operations.Gets {
		col := config.Columns[colName]
		result += fmt.Sprintf(`
	var old%s %s
	if existing.%s != entity.%s {
		old%s = existing.%s
	}
		`, PascalCaser(colName), toGoType(col.Type, col.AllowNull), PascalCaser(colName), PascalCaser(colName),
			PascalCaser(colName), PascalCaser(colName))
	}

	return result
}

func invalidateUniqueColumnsCache(config EntityConfig) string {
	result := ""

	for _, colName := range config.Operations.Gets {
		result += fmt.Sprintf(`
	if old%s != "" {
		oldCacheKey := fmt.Sprintf("user_%s:%%s", old%s)
		d.cache.Delete(oldCacheKey)
		d.cacheProvider.InvalidateCache("%s", oldCacheKey)
	}`, PascalCaser(colName), SnakeCaser(colName), PascalCaser(colName), SnakeCaser(config.Name))
	}

	return result
}
