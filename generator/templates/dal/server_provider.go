package dal

import (
	"database/sql"
	"fmt"
	"io/ioutil"
	"math/rand"
	"os"
	"regexp"
	"time"

	_ "github.com/go-sql-driver/mysql"
	"gopkg.in/yaml.v3"
)

// --------------------------------------------------------------------
// 1. DATA STRUCTURES FOR CONFIG
// --------------------------------------------------------------------

// Config holds a map of entityName -> EntityConfig
type Config struct {
	Entities map[string]*EntityConfig `yaml:"entities"`
}

// EntityConfig can represent either a non-sharded entity (Instances) or a sharded entity (Shards).
type EntityConfig struct {
	Instances *InstancesConfig `yaml:"instances,omitempty"`
	Shards    []ShardConfig    `yaml:"shards,omitempty"`
}

// InstancesConfig for non-sharded entities
type InstancesConfig struct {
	Reads  []InstanceConfig `yaml:"reads"`
	Writes []InstanceConfig `yaml:"writes"`
}

// ShardConfig for sharded entities
type ShardConfig struct {
	Name   string           `yaml:"name"`
	Reads  []InstanceConfig `yaml:"reads,omitempty"`
	Writes []InstanceConfig `yaml:"writes,omitempty"`
}

// InstanceConfig holds the connection details for a database instance
type InstanceConfig struct {
	Server      string      `yaml:"server"`
	Database    string      `yaml:"database"`
	Credentials Credentials `yaml:"credentials"`
}

// Credentials may reference environment variables (e.g. ${DB_PASS})
type Credentials struct {
	User string `yaml:"user"`
	Pass string `yaml:"pass"`
}

// --------------------------------------------------------------------
// 2. OPERATION TYPES
// --------------------------------------------------------------------
type OperationType int

const (
	OperationRead OperationType = iota
	OperationWrite
)

// --------------------------------------------------------------------
// 3. SERVER PROVIDER INTERFACE
// --------------------------------------------------------------------
type ServerProvider interface {
	// Return a database connection for a given operation, entityName, and id (for potential sharding).
	GetDatabase(op OperationType, entityName string, id int64) (*sql.DB, error)
	// Connect initializes all required DB connections.
	Connect() error
}

// --------------------------------------------------------------------
// 4. DEFAULT IMPLEMENTATION OF SERVER PROVIDER
// --------------------------------------------------------------------
type DefaultServerProvider struct {
	config *Config
	dbMap  map[string]*sql.DB // Cache: key is "server|database"
}

// NewDefaultServerProvider constructs a provider by loading config from file,
// expanding environment variables, but NOT automatically opening connections yet.
func NewDefaultServerProvider(configPath string) (*DefaultServerProvider, error) {
	// 1. Read YAML file
	data, err := ioutil.ReadFile(configPath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %v", err)
	}

	// 2. Parse YAML into Config struct
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("failed to parse YAML: %v", err)
	}

	// 3. Expand environment variables in credentials
	expandEnvInConfig(&cfg)

	// 4. Initialize the provider with an empty map.
	provider := &DefaultServerProvider{
		config: &cfg,
		dbMap:  make(map[string]*sql.DB),
	}

	return provider, nil
}

// Connect eagerly opens connections for all known instances (reads/writes) for each entity
// and stores them in dbMap.
func (p *DefaultServerProvider) Connect() error {
	for entityName, entityCfg := range p.config.Entities {
		if entityCfg.Instances != nil {
			// Non-sharded
			// Connect all reads
			for _, inst := range entityCfg.Instances.Reads {
				if err := p.openDBConnection(entityName, inst); err != nil {
					return err
				}
			}
			// Connect all writes
			for _, inst := range entityCfg.Instances.Writes {
				if err := p.openDBConnection(entityName, inst); err != nil {
					return err
				}
			}
		}

		// Sharded
		for _, shard := range entityCfg.Shards {
			// Connect all reads
			for _, inst := range shard.Reads {
				if err := p.openDBConnection(entityName+"-"+shard.Name, inst); err != nil {
					return err
				}
			}
			// Connect all writes
			for _, inst := range shard.Writes {
				if err := p.openDBConnection(entityName+"-"+shard.Name, inst); err != nil {
					return err
				}
			}
		}
	}
	return nil
}

// openDBConnection checks if we've already opened a connection for this server+db key,
// otherwise it opens a new one and stores it in dbMap.
func (p *DefaultServerProvider) openDBConnection(debugLabel string, inst InstanceConfig) error {
	key := fmt.Sprintf("%s|%s", inst.Server, inst.Database)

	if _, exists := p.dbMap[key]; exists {
		// already opened
		return nil
	}

	dsn := fmt.Sprintf("%s:%s@tcp(%s)/%s?parseTime=true",
		inst.Credentials.User,
		inst.Credentials.Pass,
		inst.Server,
		inst.Database,
	)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return fmt.Errorf("failed to open db connection for %s: %v", debugLabel, err)
	}

	// Optional: ping to verify connectivity
	if err := db.Ping(); err != nil {
		return fmt.Errorf("db ping failed for %s: %v", debugLabel, err)
	}

	p.dbMap[key] = db
	return nil
}

// GetDatabase implements the ServerProvider interface.
func (p *DefaultServerProvider) GetDatabase(op OperationType, entityName string, id int64) (*sql.DB, error) {
	// 1. Find the entity config by name from the map
	entityCfg, ok := p.config.Entities[entityName]
	if !ok {
		return nil, fmt.Errorf("unknown entity: %s", entityName)
	}

	// 2. Determine if it's sharded or not
	if len(entityCfg.Shards) > 0 {
		// Sharded entity using Jump Consistent Hash
		shardIndex := jumpConsistentHash(uint64(id), len(entityCfg.Shards))
		shardCfg := entityCfg.Shards[shardIndex]

		switch op {
		case OperationRead:
			return p.pickInstanceDB(shardCfg.Reads)
		case OperationWrite:
			return p.pickInstanceDB(shardCfg.Writes)
		}
	} else if entityCfg.Instances != nil {
		// Non-sharded entity
		switch op {
		case OperationRead:
			return p.pickInstanceDB(entityCfg.Instances.Reads)
		case OperationWrite:
			return p.pickInstanceDB(entityCfg.Instances.Writes)
		}
	}

	return nil, fmt.Errorf("no matching instance found for entity=%s, op=%v", entityName, op)
}

// jumpConsistentHash implements the "Jump Consistent Hash" algorithm by John Lamping and Eric Veach.
// Reference: https://arxiv.org/abs/1406.2294
func jumpConsistentHash(key uint64, numBuckets int) int {
	var b int64 = -1
	var j int64 = 0

	for j < int64(numBuckets) {
		b = j
		key = key*2862933555777941757 + 1
		j = int64(float64(j+1) * (float64(1<<31) / float64((key>>33)+1)))
	}
	return int(b)
}

// pickInstanceDB picks an instance from a slice (e.g. random) and returns the *sql.DB
// from the dbMap, which was already opened in Connect().
func (p *DefaultServerProvider) pickInstanceDB(instances []InstanceConfig) (*sql.DB, error) {
	if len(instances) == 0 {
		return nil, fmt.Errorf("no database instances configured in this shard/instance set")
	}

	// Random pick for demonstration (could do round-robin, etc.)
	rand.Seed(time.Now().UnixNano())
	chosen := instances[rand.Intn(len(instances))]

	// The unique key for dbMap
	key := fmt.Sprintf("%s|%s", chosen.Server, chosen.Database)

	db, exists := p.dbMap[key]
	if !exists {
		return nil, fmt.Errorf("dbMap: no open connection found for key: %s", key)
	}

	return db, nil
}

// --------------------------------------------------------------------
// 5. UTILITY: EXPAND ENV VARIABLES IN CREDENTIALS
// --------------------------------------------------------------------
func expandEnvInConfig(cfg *Config) {
	// Iterate over each entity in the map
	for _, entity := range cfg.Entities {
		// Non-sharded
		if entity.Instances != nil {
			for i := range entity.Instances.Reads {
				expandCredentials(&entity.Instances.Reads[i].Credentials)
			}
			for i := range entity.Instances.Writes {
				expandCredentials(&entity.Instances.Writes[i].Credentials)
			}
		}
		// Sharded
		for s := range entity.Shards {
			shard := &entity.Shards[s]
			for i := range shard.Reads {
				expandCredentials(&shard.Reads[i].Credentials)
			}
			for i := range shard.Writes {
				expandCredentials(&shard.Writes[i].Credentials)
			}
		}
	}
}

var envVarRegex = regexp.MustCompile(`\$\{([^}]+)\}`)

func expandCredentials(c *Credentials) {
	c.User = expandEnv(c.User)
	c.Pass = expandEnv(c.Pass)
}

func expandEnv(value string) string {
	// Replace all occurrences of ${VAR} with the environment variable
	matches := envVarRegex.FindAllStringSubmatch(value, -1)
	for _, match := range matches {
		if len(match) < 2 {
			continue
		}
		envVarName := match[1]
		envVarValue := os.Getenv(envVarName)
		// Replace *all* occurrences of ${envVarName} in `value`
		value = envVarRegex.ReplaceAllString(value, envVarValue)
	}
	return value
}
