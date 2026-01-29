package gorm

import (
	"fmt"
	"sync"
	"time"

	dbconfig "github.com/tigerroll/surfin/pkg/batch/adaptor/database/config"
	"github.com/tigerroll/surfin/pkg/batch/core/adaptor"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	"github.com/mitchellh/mapstructure"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	"gorm.io/gorm"
)

// DialectorFactory generates a gorm.Dialector from a dbconfig.DatabaseConfig.
type DialectorFactory func(cfg dbconfig.DatabaseConfig) (gorm.Dialector, error)

var (
	dialectorRegistry = make(map[string]DialectorFactory)
	dialectorMutex    sync.RWMutex
)

// RegisterDialector registers a DialectorFactory for the given database type.
func RegisterDialector(dbType string, factory DialectorFactory) {
	dialectorMutex.Lock()
	defer dialectorMutex.Unlock()
	if _, exists := dialectorRegistry[dbType]; exists {
		logger.Warnf("Dialector for type '%s' already registered. Overwriting.", dbType)
	}
	dialectorRegistry[dbType] = factory
}

// GetDialectorFactory retrieves the DialectorFactory corresponding to the specified DB type.
func GetDialectorFactory(dbType string) (DialectorFactory, error) {
	dialectorMutex.RLock()
	defer dialectorMutex.RUnlock()
	factory, ok := dialectorRegistry[dbType]
	if !ok {
		return nil, fmt.Errorf("no dialector registered for database type: %s", dbType)
	}
	return factory, nil
}

// BaseProvider provides common functionality for DBProvider implementations.
type BaseProvider struct {
	cfg    *config.Config
	dbType string
	// Map to hold connections managed by this provider (name -> DBConnection)
	connections map[string]adaptor.DBConnection
	mu          sync.RWMutex
}

// NewBaseProvider creates a new BaseProvider.
func NewBaseProvider(cfg *config.Config, dbType string) *BaseProvider {
	return &BaseProvider{
		cfg:         cfg,
		dbType:      dbType,
		connections: make(map[string]adaptor.DBConnection),
	}
}

// Type returns the database type.
func (p *BaseProvider) Type() string {
	return p.dbType
}

// GetConnection retrieves an existing connection or establishes a new one.
func (p *BaseProvider) GetConnection(name string) (adaptor.DBConnection, error) {
	p.mu.RLock()
	conn, ok := p.connections[name]
	p.mu.RUnlock()

	if ok {
		return conn, nil
	}

	// Connection not found, establish a new one
	p.mu.Lock()
	defer p.mu.Unlock()

	// Double check (DCL)
	conn, ok = p.connections[name]
	if ok {
		return conn, nil
	}

	return p.createAndStoreConnection(name)
}

// createAndStoreConnection establishes a new connection and stores it in the map.
func (p *BaseProvider) createAndStoreConnection(name string) (adaptor.DBConnection, error) {
	var dbConfig dbconfig.DatabaseConfig
	rawConfig, ok := p.cfg.Surfin.AdaptorConfigs[name]
	if !ok {
		return nil, fmt.Errorf("database configuration '%s' not found in adaptor.database configs", name)
	}
	if err := mapstructure.Decode(rawConfig, &dbConfig); err != nil {
		return nil, fmt.Errorf("failed to decode database config for '%s': %w", name, err)
	}

	// Check if the configuration type matches the provider type (or is redshift handled by postgres provider)
	if dbConfig.Type != p.dbType && !(p.dbType == "postgres" && dbConfig.Type == "redshift") {
		return nil, fmt.Errorf("provider type mismatch: expected '%s', got '%s' for connection '%s'", p.dbType, dbConfig.Type, name)
	}

	gormDB, err := p.connect(dbConfig)
	if err != nil {
		return nil, err
	}

	conn := NewGormDBAdapter(gormDB, dbConfig, name)
	p.connections[name] = conn
	logger.Infof("Established new DB connection: %s (%s)", name, p.dbType)

	return conn, nil
}

// ForceReconnect attempts to close and reopen a connection if it exists.
func (p *BaseProvider) ForceReconnect(name string) (adaptor.DBConnection, error) {
	p.mu.Lock()
	defer p.mu.Unlock()

	// 1. If an existing connection instance exists, close it.
	if existingConn, ok := p.connections[name]; ok {
		if err := existingConn.Close(); err != nil {
			logger.Warnf("Failed to close existing connection '%s' before reconnect: %v", name, err)
		}
	}

	// 2. Create and store a new connection. This will overwrite the old entry in the map if it existed.
	conn, err := p.createAndStoreConnection(name)
	if err != nil {
		return nil, err
	}

	logger.Infof("Re-established DB connection: %s (%s)", name, p.dbType)

	return conn, nil
}

// connect establishes a GORM connection based on DatabaseConfig.
func (p *BaseProvider) connect(dbConfig dbconfig.DatabaseConfig) (*gorm.DB, error) {

	// Retrieves the DialectorFactory and uses it to create a Dialector.
	dialectorFactory, err := GetDialectorFactory(dbConfig.Type)
	if err != nil {
		return nil, fmt.Errorf("failed to get dialector factory for %s: %w", dbConfig.Type, err)
	}
	dialector, err := dialectorFactory(dbConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to create dialector for %s: %w", dbConfig.Type, err)
	}

	// Fixes GORM's logging level to config.LogLevelSilent.
	gormLogLevel := config.LogLevelSilent

	gormConfig := &gorm.Config{
		Logger: NewGormLogger(string(gormLogLevel)),
	}

	db, err := gorm.Open(dialector, gormConfig)
	if err != nil {
		return nil, fmt.Errorf("failed to open GORM connection: %w", err)
	}

	sqlDB, err := db.DB()
	if err != nil {
		return nil, fmt.Errorf("failed to get underlying sql.DB: %w", err)
	}

	// Apply pool settings
	sqlDB.SetMaxOpenConns(dbConfig.Pool.MaxOpenConns)
	sqlDB.SetMaxIdleConns(dbConfig.Pool.MaxIdleConns)
	if dbConfig.Pool.ConnMaxLifetimeMinutes > 0 {
		sqlDB.SetConnMaxLifetime(time.Duration(dbConfig.Pool.ConnMaxLifetimeMinutes) * time.Minute)
	}

	return db, nil
}

// CloseAll closes all connections managed by this provider.
func (p *BaseProvider) CloseAll() error {
	p.mu.Lock()
	defer p.mu.Unlock()

	var lastErr error
	for name, conn := range p.connections {
		if err := conn.Close(); err != nil {
			logger.Errorf("Failed to close connection '%s': %v", name, err)
			lastErr = err
		}
		delete(p.connections, name)
	}
	return lastErr
}

// --- Concrete Provider Implementations ---

// PostgresDBProvider handles PostgreSQL and Redshift connections.
type PostgresDBProvider struct {
	*BaseProvider
}

// NewPostgresProvider creates a new PostgreSQL DBProvider.
func NewPostgresProvider(cfg *config.Config) adaptor.DBProvider {
	return &PostgresDBProvider{BaseProvider: NewBaseProvider(cfg, "postgres")}
}

func (p *PostgresDBProvider) ConnectionString(c dbconfig.DatabaseConfig) string {
	// Adjust to the DSN format expected by GORM (gorm.io/driver/postgres)
	return fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=%s",
		c.Host, c.Port, c.User, c.Password, c.Database, c.Sslmode)
}

// MySQLDBProvider handles MySQL connections.
type MySQLDBProvider struct {
	*BaseProvider
}

func NewMySQLProvider(cfg *config.Config) adaptor.DBProvider {
	return &MySQLDBProvider{BaseProvider: NewBaseProvider(cfg, "mysql")}
}

func (p *MySQLDBProvider) ConnectionString(c dbconfig.DatabaseConfig) string {
	// DSN format expected by GORM (gorm.io/driver/mysql)
	// user:password@tcp(host:port)/dbname?charset=utf8mb4&parseTime=True&loc=Local
	var authPart string
	if c.User != "" {
		authPart = c.User
		if c.Password != "" {
			authPart = fmt.Sprintf("%s:%s", c.User, c.Password)
		}
		authPart += "@" // Add @ if username is present
	}

	// Construct the DSN
	return fmt.Sprintf("%stcp(%s:%d)/%s?charset=utf8mb4&parseTime=True&loc=Local",
		authPart, c.Host, c.Port, c.Database)
}

// SQLiteDBProvider handles SQLite connections.
type SQLiteDBProvider struct {
	*BaseProvider
}

func NewSQLiteProvider(cfg *config.Config) adaptor.DBProvider {
	return &SQLiteDBProvider{BaseProvider: NewBaseProvider(cfg, "sqlite")}
}

func (p *SQLiteDBProvider) ConnectionString(c dbconfig.DatabaseConfig) string {
	// GORM SQLite Dialector expects the file path directly
	return c.Database
}

// --- Test Utility Functions ---

// GetProviderForTest retrieves the appropriate concrete DBProvider instance for testing purposes.
func GetProviderForTest(cfg dbconfig.DatabaseConfig) (adaptor.DBProvider, error) {
	// Note: We need a dummy config.Config instance to initialize the providers,
	// as they expect it for BaseProvider initialization.
	dummyCfg := config.NewConfig()

	switch cfg.Type {
	case "postgres", "redshift":
		return NewPostgresProvider(dummyCfg), nil
	case "mysql":
		return NewMySQLProvider(dummyCfg), nil
	case "sqlite":
		return NewSQLiteProvider(dummyCfg), nil
	default:
		return nil, fmt.Errorf("unsupported database type for test: %s", cfg.Type)
	}
}

// GetConnectionStringForTest retrieves the connection string for a given DatabaseConfig.
// This is intended for use in configuration tests where a full connection is not needed. (This function is not used in the current code, but it is kept for future use.)
func GetConnectionStringForTest(cfg dbconfig.DatabaseConfig) (string, error) {
	provider, err := GetProviderForTest(cfg)
	if err != nil {
		return "", err
	}

	// ConnectionString method is defined on concrete providers.
	switch p := provider.(type) {
	case *PostgresDBProvider:
		return p.ConnectionString(cfg), nil
	case *MySQLDBProvider:
		return p.ConnectionString(cfg), nil
	case *SQLiteDBProvider:
		return p.ConnectionString(cfg), nil
	default:
		return "", fmt.Errorf("internal error: provider type %T does not support ConnectionString access", p)
	}
}

// GetDialectorForTest retrieves the GORM Dialector for a given DatabaseConfig.
// This is intended for use in repository tests where a GORM Dialector is needed without full provider setup. (This function is not used in the current code, but it is kept for future use.)
func GetDialectorForTest(cfg dbconfig.DatabaseConfig) (gorm.Dialector, error) {
	dialectorFactory, err := GetDialectorFactory(cfg.Type)
	if err != nil {
		return nil, err
	}
	return dialectorFactory(cfg)
}
