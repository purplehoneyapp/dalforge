package generator

import (
	"embed"
	"fmt"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

//go:embed templates/*
var templateFS embed.FS

type Generator struct {
	dalTemplate *template.Template
	sqlTemplate *template.Template
}

func NewGenerator() (*Generator, error) {
	funcMap := template.FuncMap{
		"toLower":    strings.ToLower,
		"snakeCase":  SnakeCaser,
		"pascalCase": PascalCaser,
		"camelCase":  CamelCaser,
		"goField":    PascalCaser,
		"goColumn":   toColumnName,
		"toGoType":   toGoType,
		"toSQLType":  toSQLType,
		"dict":       dict,
		"join":       join,
		"keys":       keys,
		"sub":        sub,
		"add":        add,
	}

	dalTmpl, err := template.New("dal").Funcs(funcMap).ParseFS(templateFS, "templates/dal/*.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to parse DAL template: %w", err)
	}

	sqlTmpl, err := template.New("sql").Funcs(funcMap).ParseFS(templateFS, "templates/sql/*.tmpl")
	if err != nil {
		return nil, fmt.Errorf("failed to parse SQL template: %w", err)
	}

	return &Generator{
		dalTemplate: dalTmpl,
		sqlTemplate: sqlTmpl,
	}, nil
}

func (g *Generator) GenerateDAL(yamlInput string) (string, error) {
	config, err := g.parseYAML(yamlInput)
	if err != nil {
		return "", err
	}

	var buf strings.Builder
	if err := g.dalTemplate.ExecuteTemplate(&buf, "base.tmpl", config); err != nil {
		return "", fmt.Errorf("DAL generation failed: %w", err)
	}
	return buf.String(), nil
}

func (g *Generator) parseYAML(yamlInput string) (EntityConfig, error) {
	var config EntityConfig
	if err := yaml.Unmarshal([]byte(yamlInput), &config); err != nil {
		return EntityConfig{}, fmt.Errorf("YAML parsing failed: %w", err)
	}

	if config.Name == "" {
		return EntityConfig{}, fmt.Errorf("entity name is required")
	}

	// Validate required operations
	if len(config.Operations.Gets) == 0 {
		return EntityConfig{}, fmt.Errorf("at least one get operation must be specified")
	}

	// Add default columns (id, created, updated are handled in templates)
	// Validate user columns don't conflict with defaults
	for colName := range config.Columns {
		if colName == "id" || colName == "created" || colName == "updated" {
			return EntityConfig{}, fmt.Errorf("column name '%s' is reserved", colName)
		}
	}

	return config, nil
}

type EntityConfig struct {
	Name       string            `yaml:"name"`
	Version    string            `yaml:"version"`
	Columns    map[string]Column `yaml:"columns"`
	Operations OperationConfig   `yaml:"operations"`
	Caching    CachingConfig     `yaml:"caching"`
}

type CachingConfig struct {
	Type                    string `yaml:"type"`
	SingleExpirationSeconds int32  `yaml:"singleExpirationSeconds"`
	ListExpirationSeconds   int32  `yaml:"listExpirationSeconds"`
}

type Column struct {
	Type      string `yaml:"type"`
	AllowNull bool   `yaml:"allowNull"`
	Unique    bool   `yaml:"unique"`
}

type OperationConfig struct {
	Gets   []string `yaml:"gets"`
	Lists  []string `yaml:"lists"`
	Store  bool     `yaml:"store"`
	Delete bool     `yaml:"delete"`
}
