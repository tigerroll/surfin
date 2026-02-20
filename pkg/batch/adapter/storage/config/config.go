package config

// StorageConfig holds configuration for a single storage connection.
type StorageConfig struct {
	Type            string `yaml:"type"`             // Type of storage (e.g., "gcs", "s3", "local", "ftp", "sftp").
	BucketName      string `yaml:"bucket_name"`      // Default bucket name for operations.
	CredentialsFile string `yaml:"credentials_file"` // Path to credentials file (e.g., service account key for GCS).
	BaseDir         string `yaml:"base_dir"`         // Base directory for local file system operations.
}

// DatasourcesConfig holds a map of named storage configurations.
type DatasourcesConfig map[string]StorageConfig
