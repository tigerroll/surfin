CREATE TABLE IF NOT EXISTS batch_job_instance (
    id TEXT PRIMARY KEY,
    job_name TEXT NOT NULL,
    parameters TEXT NOT NULL,
    create_time DATETIME NOT NULL,
    version INTEGER NOT NULL,
    parameters_hash TEXT NOT NULL
);

CREATE UNIQUE INDEX IF NOT EXISTS idx_job_instance_unique ON batch_job_instance (job_name, parameters_hash);
