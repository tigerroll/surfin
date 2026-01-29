package config

// PoolConfig holds database connection pool settings.
type PoolConfig struct {
	MaxOpenConns           int `yaml:"max_open_conns"`
	MaxIdleConns           int `yaml:"max_idle_conns"`
	ConnMaxLifetimeMinutes int `yaml:"conn_max_lifetime_minutes"`
}

// DatabaseConfig holds database connection settings.
type DatabaseConfig struct {
	Type      string     `yaml:"type"`                 // Database type (e.g., "postgres", "mysql", "sqlite").
	Host      string     `yaml:"host"`                 // Database host address.
	Port      int        `yaml:"port"`                 // Database port number.
	Database  string     `yaml:"database"`             // Database name.
	User      string     `yaml:"user"`                 // Database user.
	Password  string     `yaml:"password"`             // Database password.
	Schema    string     `yaml:"schema,omitempty"`     // Schema name for PostgreSQL/Redshift.
	Sslmode   string     `yaml:"sslmode"`              // SSL mode for the connection.
	ProjectID string     `yaml:"project_id,omitempty"` // Project ID for Google Cloud databases.
	DatasetID string     `yaml:"dataset_id,omitempty"` // Dataset ID for Google Cloud databases.
	TableID   string     `yaml:"table_id,omitempty"`   // Table ID for Google Cloud databases.
	Pool      PoolConfig `yaml:"pool"`                 // Connection pool settings.
}
