package gorm

import (
	"fmt"
	"sync"
	"time"

	"github.com/mitchellh/mapstructure"
	dbconfig "github.com/tigerroll/surfin/pkg/batch/adapter/database/config"
	"github.com/tigerroll/surfin/pkg/batch/core/adapter"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
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
	connections map[string]adapter.DBConnection
	mu          sync.RWMutex
}

// NewBaseProvider creates a new BaseProvider.
func NewBaseProvider(cfg *config.Config, dbType string) *BaseProvider {
	return &BaseProvider{
		cfg:         cfg,
		dbType:      dbType,
		connections: make(map[string]adapter.DBConnection),
	}
}

// Type returns the database type.
func (p *BaseProvider) Type() string {
	return p.dbType
}

// GetConnection retrieves an existing connection or establishes a new one.
func (p *BaseProvider) GetConnection(name string) (adapter.DBConnection, error) {
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
func (p *BaseProvider) createAndStoreConnection(name string) (adapter.DBConnection, error) {
	var dbConfig dbconfig.DatabaseConfig
	rawConfig, ok := p.cfg.Surfin.AdapterConfigs[name]
	if !ok {
		return nil, fmt.Errorf("database configuration '%s' not found in adapter.database configs", name)
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
func (p *BaseProvider) ForceReconnect(name string) (adapter.DBConnection, error) {
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
