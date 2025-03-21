package generator

import (
	"crypto/sha256"
	"embed"
	"encoding/hex"
	"fmt"
	"io"
	"io/fs"
	"strings"
	"text/template"

	"gopkg.in/yaml.v3"
)

//go:embed templates/*
//go:embed templates/**/*
var templateFS embed.FS

type Generator struct {
	dalTemplate *template.Template
	sqlTemplate *template.Template
}

func NewGenerator() (*Generator, error) {
	funcMap := template.FuncMap{
		"toLower":                      strings.ToLower,
		"snakeCase":                    SnakeCaser,
		"pascalCase":                   PascalCaser,
		"camelCase":                    CamelCaser,
		"goField":                      PascalCaser,
		"goColumn":                     toColumnName,
		"toGoType":                     toGoType,
		"toSQLType":                    toSQLType,
		"dict":                         dict,
		"join":                         join,
		"keys":                         keys,
		"sub":                          sub,
		"add":                          add,
		"querySelect":                  querySelect,
		"goFuncCallParameters":         goFuncCallParameters,
		"listQuery":                    listQuery,
		"listQueryParams":              listQueryParams,
		"listFuncParams":               listFuncParams,
		"listCacheKey":                 listCacheKey,
		"listSQLIndexes":               listSQLIndexes,
		"checkColumnsChanged":          checkColumnsChanged,
		"invalidateUniqueColumnsCache": invalidateUniqueColumnsCache,
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

	config.TemplateVersion, err = getDirectoryHash(templateFS, ".")
	if err != nil {
		return "", err
	}

	// This builds entity DAL .go file.
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

	// Add default columns (id, created, updated are handled in templates)
	// Validate user columns don't conflict with defaults
	for colName := range config.Columns {
		if colName == "id" || colName == "created" || colName == "updated" {
			return EntityConfig{}, fmt.Errorf("column name '%s' is reserved", colName)
		}
	}

	// Set default value for list invalidation
	if config.Caching.ListInvalidation == "" {
		config.Caching.ListInvalidation = "flush"
	}

	err := ValidateEntityConfig(config)

	return config, err
}

func getDirectoryHash(efs embed.FS, inputDir string) (string, error) {
	hash := sha256.New()

	// Use fs.WalkDir from the "io/fs" package to traverse the embedded FS.
	err := fs.WalkDir(efs, inputDir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		// Skip directories
		if d.IsDir() {
			return nil
		}

		// Open the file from the embedded FS
		file, err := efs.Open(path)
		if err != nil {
			return err
		}
		defer file.Close()

		// Copy the file contents into the hash
		if _, err := io.Copy(hash, file); err != nil {
			return err
		}

		return nil
	})

	if err != nil {
		return "", fmt.Errorf("failed to compute directory hash: %w", err)
	}

	// Finalize the hash and return it as a hex string
	return hex.EncodeToString(hash.Sum(nil)), nil
}

type EntityConfig struct {
	TemplateVersion string               // This item is not loaded from yaml but is calculated at runtime
	Name            string               `yaml:"name"`
	Version         string               `yaml:"version"`
	Columns         map[string]Column    `yaml:"columns"`
	Operations      OperationConfig      `yaml:"operations"`
	Caching         CachingConfig        `yaml:"caching"`
	CircuitBreaker  CircuitBreakerConfig `yaml:"circuitbreaker"`
}

type CachingConfig struct {
	Type                    string `yaml:"type"`
	SingleExpirationSeconds int32  `yaml:"singleExpirationSeconds"`
	ListExpirationSeconds   int32  `yaml:"listExpirationSeconds"`
	ListInvalidation        string `yaml:"listInvalidation"`
	MaxItemsCount           int32  `yaml:"maxItemsCount"`
}

type CircuitBreakerConfig struct {
	TimeoutSeconds      int32 `yaml:"timeoutSeconds"`
	ConsecutiveFailures int32 `yaml:"consecutiveFailures"`
}

type Column struct {
	Type      string `yaml:"type"`
	AllowNull bool   `yaml:"allowNull"`
	Unique    bool   `yaml:"unique"`
}

type OperationConfig struct {
	Gets   []string     `yaml:"gets"`
	Lists  []ListConfig `yaml:"lists"`
	Store  bool         `yaml:"store"`
	Delete bool         `yaml:"delete"`
}

type ListConfig struct {
	Name       string `yaml:"name"`
	Where      string `yaml:"where"`
	Order      string `yaml:"order"`
	Descending bool   `yaml:"desc"`
}
