package generator

import (
	"fmt"
	"regexp"
	"sort"
	"strings"
)

func querySelect(columns map[string]Column) string {
	result := "id, version, "

	var names []string
	for name := range columns {
		names = append(names, name)
	}
	sort.Strings(names) // Sort alphabetically

	for _, name := range names {
		result = result + fmt.Sprintf("%s, ", SnakeCaser(name))
	}

	result += "created, updated"
	return result
}

// if given input: "deleted = 0 AND target = :target AND age = :age"
// gives output: ["target", "age"]
func extractParams(input string) []string {
	// Regular expression to match placeholders starting with ':'
	re := regexp.MustCompile(`:(\w+)`)

	// Find all matches
	matches := re.FindAllStringSubmatch(input, -1)

	// Extract parameter names
	var params []string
	for _, match := range matches {
		if len(match) > 1 {
			params = append(params, match[1])
		}
	}

	return params
}

// replaceParams replaces all named parameters (e.g., :target) with "?" for SQL placeholders.
func replaceParams(query string) string {
	re := regexp.MustCompile(`:(\w+)`)
	return re.ReplaceAllString(query, "?")
}

/*
	Makes the where part of this query.

query = SELECT * FROM `posts` WHERE

	`deleted` = 0 and `age` = ?  # if where != nil; validate those columns are valid and indexed properly
	AND id < ?  # Pagination if descending is true and startId != 0
	AND id > ?  # Pagination if descending is false and startId != 0
*/
func listWhereQuery(isStartIdZero bool, list ListConfig) string {
	result := ""
	// if we have custom where clause
	if strings.TrimSpace(list.Where) != "" {
		if result != "" {
			result += " AND " + replaceParams(list.Where)
		} else {
			result += replaceParams(list.Where)
		}
	}

	if list.Descending && !isStartIdZero && strings.TrimSpace(list.Order) != "" {
		if result != "" {
			result += " AND "
		}
		result += "id < ?"
	} else if !list.Descending && !isStartIdZero {
		if result != "" {
			result += " AND "
		}
		result += "id > ?"
	}

	return result
}

func listQuery(isStartIdZero bool, entityName string, list ListConfig, columns map[string]Column) string {
	result := ""
	where := listWhereQuery(isStartIdZero, list)

	result = fmt.Sprintf("SELECT %s FROM %ss", querySelect(columns), SnakeCaser(entityName))
	if where != "" {
		result += fmt.Sprintf(" WHERE %s", where)
	}

	if strings.TrimSpace(list.Order) != "" {
		result += fmt.Sprintf(" ORDER BY %s", list.Order)
		if list.Descending {
			result += " DESC, id DESC"
		} else {
			result += ", id"
		}
	} else {
		result += " ORDER BY id"
	}

	result += " LIMIT ?"
	return result
}

// Input: "email", { allowNulls: true, forceLowercase: true }
// Output: strings.ToLower(email.String)
//
// Input: "email", { allowNulls: true, forceLowercase: false }
// Output: email
func columnParameter(structNamePrefix string, colName string, col Column) string {
	if structNamePrefix != "" {
		structNamePrefix = structNamePrefix + "."
	}

	return fmt.Sprintf("%s%s", structNamePrefix, PascalCaser(colName))
}

// Outputs string that is used for this:
// rows, err := db.QueryContext(ctx, query, {{queryParams .Columns}})
// should created something like this as output:
// rows, err := db.QueryContext(ctx, query, structNamePrefix.email, structNamePrefix.age, structNamePrefix.pageSize)
func goFuncCallParameters(structNamePrefix string, columns map[string]Column) string {
	var result []string
	keys := make([]string, 0, len(columns))

	// Extract keys
	for colName := range columns {
		keys = append(keys, colName)
	}

	// Sort keys alphabetically
	sort.Strings(keys)

	// Process in sorted order
	for _, colName := range keys {
		result = append(result, columnParameter(structNamePrefix, colName, columns[colName]))
	}

	return strings.Join(result, ", ")
}

// Outputs string that is used for this:
// rows, err := db.QueryContext(ctx, query, {{listQueryParams .List}})
// should created something like this as output:
// rows, err := db.QueryContext(ctx, query, startID, age, pageSize)
func listQueryParams(isStartIdZero bool, list ListConfig, columns map[string]Column) string {
	result := ""
	params := extractParams(list.Where)
	for _, param := range params {
		result += fmt.Sprintf("%s, ", CamelCaser(param))
	}

	if !isStartIdZero {
		result += "startID, pageSize"
	} else {
		result += "pageSize"
	}

	return result
}

// for input:
// func (d *UserDAL) listByAge(ctx context.Context, {{listFuncParams .List .Root.Columns}}) ([]*User, error) {
// outputs:
// func (d *UserDAL) listByAge(ctx context.Context, age int, startID int64, pageSize int) ([]*User, error) {
func listFuncParams(list ListConfig, columns map[string]Column) (string, error) {
	result := ""
	params := extractParams(list.Where)
	for _, param := range params {
		col, ok := columns[param]
		if !ok {
			return "", fmt.Errorf("dal yaml definition error. missing column %s, which is used in where under list %s", param, list.Name)
		}
		result += fmt.Sprintf("%s %s, ", CamelCaser(param), toGoType(col.Type, col.AllowNull))
	}

	result += "startID int64, pageSize int"
	return result, nil
}

// Create cache key similar to this:
// fmt.Sprintf("{{$entityTableName}}_{{.List.Name | snakeCase}}:%v:%d:%d", age, startID, pageSize)
func listCacheKey(entityName string, list ListConfig, columns map[string]Column) string {
	result := ""
	key := fmt.Sprintf("%s_%s", SnakeCaser(entityName), SnakeCaser(list.Name))
	params := extractParams(list.Where)
	for range params {
		key += ":%v"
	}
	key += ":%d:%d"

	result = fmt.Sprintf(`fmt.Sprintf("%s", %s)`, key, listQueryParams(false, list, columns))
	return result
}
