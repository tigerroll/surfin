CREATE TABLE IF NOT EXISTS batch_job_instance (
    id VARCHAR(36) PRIMARY KEY,
    job_name VARCHAR(255) NOT NULL,
    parameters TEXT NOT NULL,
    create_time TIMESTAMP WITH TIME ZONE NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version INTEGER NOT NULL DEFAULT 0,
    parameters_hash VARCHAR(64) NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_job_instance_unique ON batch_job_instance (job_name, parameters_hash);
