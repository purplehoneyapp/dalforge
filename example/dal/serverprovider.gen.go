package dal

import (
	"database/sql"
	"fmt"
	"math/rand"
	"strings"

	_ "github.com/go-sql-driver/mysql" // or another driver if desired
)

// DBProvider defines the interface your ServerProvider must implement.
type DBProvider interface {
	GetDatabase(entityName string, isWriteOperation bool) (*sql.DB, error)
	AllDatabases(entityName string, mode string) []*sql.DB
	Connect() error
	Disconnect() error
}

// ServerProvider is an implementation of DBProvider.
type ServerProvider struct {
	config *ServerConfig
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

// ServerConfig represents the root configuration for the database topology.
type ServerConfig struct {
	ServerGroups []ServerGroupConfig `yaml:"serverGroup" json:"serverGroup"`
}

// ServerGroupConfig maps a group of entities to specific read/write database instances.
type ServerGroupConfig struct {
	Name      string          `yaml:"name" json:"name"`
	Entities  []string        `yaml:"entities" json:"entities"`
	Instances InstancesConfig `yaml:"instances" json:"instances"`
}

// InstancesConfig holds the read and write DB instances.
type InstancesConfig struct {
	Reads  []DBInstance `yaml:"reads" json:"reads"`
	Writes []DBInstance `yaml:"writes" json:"writes"`
}

// DBInstance represents one database connection (read or write).
type DBInstance struct {
	Server      string            `yaml:"server" json:"server"`
	Database    string            `yaml:"database" json:"database"`
	Credentials CredentialsConfig `yaml:"credentials" json:"credentials"`
}

// CredentialsConfig holds the user and password for a database connection.
type CredentialsConfig struct {
	User string `yaml:"user" json:"user"`
	Pass string `yaml:"pass" json:"pass"`
}

// NewServerProvider creates a new ServerProvider given a populated ServerConfig struct.
func NewServerProvider(cfg *ServerConfig) (*ServerProvider, error) {
	if cfg == nil {
		return nil, fmt.Errorf("configuration cannot be nil")
	}

	return &ServerProvider{
		config: cfg,
		groups: make(map[string]*dbGroup),
	}, nil
}

// Connect parses the configuration and connects to all DBs.
func (s *ServerProvider) Connect() error {
	// Create DB connections for each server group
	for _, group := range s.config.ServerGroups {
		dbGrp := &dbGroup{
			name:     group.Name,
			entities: group.Entities,
		}

		// Connect all read instances
		for _, inst := range group.Instances.Reads {
			dbConn, err := s.connectInstance(inst)
			if err != nil {
				return fmt.Errorf("failed to connect read instance for group %s: %w", group.Name, err)
			}
			dbGrp.reads = append(dbGrp.reads, dbConn)
		}

		// Connect all write instances
		for _, inst := range group.Instances.Writes {
			dbConn, err := s.connectInstance(inst)
			if err != nil {
				return fmt.Errorf("failed to connect write instance for group %s: %w", group.Name, err)
			}
			dbGrp.writes = append(dbGrp.writes, dbConn)
		}

		s.groups[group.Name] = dbGrp
	}

	return nil
}

// connectInstance creates a *sql.DB for the given instance.
func (s *ServerProvider) connectInstance(inst DBInstance) (*sql.DB, error) {
	// Build DataSourceName for MySQL driver.
	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true&loc=UTC",
		inst.Credentials.User,
		inst.Credentials.Pass,
		inst.Server,
		inst.Database,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, err
	}

	// Test the connection
	if err := db.Ping(); err != nil {
		db.Close()
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
// Optional mode: "read", "write", or "all".
func (s *ServerProvider) AllDatabases(entityName string, mode string) []*sql.DB {
	var result []*sql.DB

	for _, group := range s.groups {
		if contains(group.entities, entityName) {
			switch mode {
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
	grp := s.findGroupByEntity(entityName)
	if grp == nil {
		return nil, fmt.Errorf("could not find settings for entity: %s", entityName)
	}

	if isWriteOperation {
		if len(grp.writes) == 0 {
			return nil, fmt.Errorf("no write instances found for entity: %s", entityName)
		}
		// Example: always pick the first write connection
		return grp.writes[0], nil
	}

	if len(grp.reads) == 0 {
		return nil, fmt.Errorf("no read instances found for entity: %s", entityName)
	}
	// Example load-balancing: pick random read connection
	idx := rand.Intn(len(grp.reads))
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
