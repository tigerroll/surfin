CREATE TABLE IF NOT EXISTS batch_job_instance (
    id VARCHAR(36) PRIMARY KEY,
    job_name VARCHAR(255) NOT NULL,
    parameters TEXT NOT NULL,
    create_time DATETIME NOT NULL,
    version INT NOT NULL,
    parameters_hash VARCHAR(64) NOT NULL,
    UNIQUE KEY idx_job_instance_unique (job_name, parameters_hash)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
