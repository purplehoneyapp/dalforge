package generator

import (
	"fmt"
	"strings"
)

func (g *Generator) GenerateSQL(yamlInput string) (string, error) {
	config, err := g.parseYAML(yamlInput)
	if err != nil {
		return "", err
	}

	var buf strings.Builder
	if err := g.sqlTemplate.ExecuteTemplate(&buf, "sql.tmpl", config); err != nil {
		return "", fmt.Errorf("SQL generation failed: %w", err)
	}
	return buf.String(), nil
}
