CREATE TABLE IF NOT EXISTS batch_job_execution (
    id TEXT PRIMARY KEY,
    job_instance_id TEXT NOT NULL,
    job_name TEXT NOT NULL,
    parameters TEXT NOT NULL,
    start_time DATETIME NOT NULL,
    end_time DATETIME,
    status TEXT NOT NULL,
    exit_status TEXT NOT NULL,
    exit_code INTEGER NOT NULL,
    failures TEXT,
    version INTEGER NOT NULL,
    create_time DATETIME NOT NULL,
    last_updated DATETIME NOT NULL,
    execution_context TEXT,
    current_step_name TEXT,
    restart_count INTEGER NOT NULL,
    FOREIGN KEY (job_instance_id) REFERENCES batch_job_instance(id)
);

CREATE INDEX IF NOT EXISTS idx_job_instance_id ON batch_job_execution (job_instance_id);
