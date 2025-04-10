package dal

import (
	"database/sql"
	"fmt"
	"math/rand"
	"os"
	"strings"

	_ "github.com/go-sql-driver/mysql" // or another driver if desired
	"gopkg.in/yaml.v3"
)

// DBProvider defines the interface your ServerProvider must implement.
type DBProvider interface {
	// GetDatabase returns a *sql.DB given an entityName, a flag indicating if it’s a write operation,
	GetDatabase(entityName string, isWriteOperation bool) (*sql.DB, error)

	// mode is: "read", "write" or "all"
	AllDatabases(entityName string, mode string) []*sql.DB

	// Connect connects to all databases defined in the YAML configuration.
	Connect() error

	// Disconnect closes all database connections.
	Disconnect() error
}

// ServerProvider is an implementation of DBProvider.
type ServerProvider struct {
	config *serverConfig
	// groups stores references to each server group keyed by the group name.
	groups map[string]*dbGroup
}

// dbGroup holds the databases for a particular group of entities.
// We separate read and write connections.
type dbGroup struct {
	name     string
	entities []string
	reads    []*sql.DB
	writes   []*sql.DB
}

// serverConfig represents the YAML structure.
type serverConfig struct {
	ServerGroup []struct {
		Name      string   `yaml:"name"`
		Entities  []string `yaml:"entites"` // note the YAML tag is "entites" per your sample
		Instances struct {
			Reads  []dbInstance `yaml:"reads"`
			Writes []dbInstance `yaml:"writes"`
		} `yaml:"instances"`
	} `yaml:"serverGroup"`
}

// dbInstance represents one database connection (read or write).
type dbInstance struct {
	Server      string `yaml:"server"`
	Database    string `yaml:"database"`
	Credentials struct {
		User string `yaml:"user"`
		Pass string `yaml:"pass"`
	} `yaml:"credentials"`
}

// NewServerProvider creates a new ServerProvider given a path to the YAML config.
func NewServerProvider(configurationYaml string) (*ServerProvider, error) {
	// Expand environment variables like ${USER_DB_PASS}
	expandedData := os.ExpandEnv(string(configurationYaml))

	var cfg serverConfig
	if err := yaml.Unmarshal([]byte(expandedData), &cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal YAML: %w", err)
	}

	return &ServerProvider{
		config: &cfg,
		groups: make(map[string]*dbGroup),
	}, nil
}

// Connect reads the YAML file, parses it, expands environment variables, and connects to all DBs.
func (s *ServerProvider) Connect() error {

	// Create DB connections for each server group
	for _, group := range s.config.ServerGroup {
		dbGrp := &dbGroup{
			name:     group.Name,
			entities: group.Entities,
		}

		// Connect all read instances
		for _, inst := range group.Instances.Reads {
			dbConn, err := s.connectInstance(inst)
			if err != nil {
				return fmt.Errorf("failed to connect read instance: %w", err)
			}
			dbGrp.reads = append(dbGrp.reads, dbConn)
		}

		// Connect all write instances
		for _, inst := range group.Instances.Writes {
			dbConn, err := s.connectInstance(inst)
			if err != nil {
				return fmt.Errorf("failed to connect write instance: %w", err)
			}
			dbGrp.writes = append(dbGrp.writes, dbConn)
		}

		s.groups[group.Name] = dbGrp
	}

	return nil
}

// connectInstance creates a *sql.DB for the given instance.
func (s *ServerProvider) connectInstance(inst dbInstance) (*sql.DB, error) {
	// Build DataSourceName for MySQL driver. Adjust if you’re using a different driver.
	// Example: "myapp:password@tcp(readserver1.domain)/myapp?parseTime=true"
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true",
		inst.Credentials.User,
		inst.Credentials.Pass,
		inst.Server,
		inst.Database,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}
	// Optionally, you can configure db.SetMaxIdleConns, db.SetMaxOpenConns, etc.

	// Test the connection
	if err := db.Ping(); err != nil {
		return nil, err
	}

	return db, nil
}

// Disconnect closes all DBs in all groups.
func (s *ServerProvider) Disconnect() error {
	for _, group := range s.groups {
		for _, r := range group.reads {
			_ = r.Close()
		}
		for _, w := range group.writes {
			_ = w.Close()
		}
	}
	return nil
}

// AllDatabases returns all sql.DB connections associated with the given entity.
// Optional mode: "read", "write", or "all" (default).
func (s *ServerProvider) AllDatabases(entityName string, mode string) []*sql.DB {
	// Determine the connection mode filter
	connectionMode := mode

	var result []*sql.DB

	// Assuming you have access to your dbGroups (you might need to pass them as a parameter)
	for _, group := range s.groups { // Replace with your actual dbGroups access
		if contains(group.entities, entityName) {
			switch connectionMode {
			case "read":
				result = append(result, group.reads...)
			case "write":
				result = append(result, group.writes...)
			default: // "all"
				result = append(result, group.reads...)
				result = append(result, group.writes...)
			}
		}
	}

	return result
}

// Helper function to check if a string exists in a slice
func contains(slice []string, item string) bool {
	for _, s := range slice {
		if s == item {
			return true
		}
	}
	return false
}

// GetDatabase looks up the server group for the given entityName, then picks a read or write DB.
func (s *ServerProvider) GetDatabase(entityName string, isWriteOperation bool) (*sql.DB, error) {
	// 1) Find which group handles this entity
	grp := s.findGroupByEntity(entityName)
	if grp == nil {
		// Could return nil or panic if not found.
		// For production, you might want to handle this error more gracefully.
		return nil, fmt.Errorf("could not find settings for entity: %s", entityName)
	}

	// 2) Return a DB from writes if it's a write operation
	if isWriteOperation {
		if len(grp.writes) == 0 {
			return nil, fmt.Errorf("no write instances found for entity: %s", entityName)
		}
		// Example: always pick the first write connection
		return grp.writes[0], nil
	}

	// 3) Otherwise, pick from the read connections
	if len(grp.reads) == 0 {
		return nil, fmt.Errorf("no read instances found for entity: %s", entityName)
	}
	// Example load-balancing: pick index = id % len(reads)
	idx := rand.Intn(len(grp.reads)) // random index
	return grp.reads[idx], nil
}

// findGroupByEntity finds the group that has the specified entity.
func (s *ServerProvider) findGroupByEntity(entityName string) *dbGroup {
	for _, grp := range s.groups {
		for _, e := range grp.entities {
			if strings.EqualFold(e, entityName) {
				return grp
			}
		}
	}
	return nil
}
