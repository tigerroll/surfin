CREATE TABLE IF NOT EXISTS batch_step_execution (
    id TEXT PRIMARY KEY,
    step_name TEXT NOT NULL,
    job_execution_id TEXT NOT NULL,
    start_time DATETIME NOT NULL,
    end_time DATETIME,
    status TEXT NOT NULL,
    exit_status TEXT NOT NULL,
    failures TEXT,
    read_count INTEGER NOT NULL,
    write_count INTEGER NOT NULL,
    commit_count INTEGER NOT NULL,
    rollback_count INTEGER NOT NULL,
    filter_count INTEGER NOT NULL,
    skip_read_count INTEGER NOT NULL,
    skip_process_count INTEGER NOT NULL,
    skip_write_count INTEGER NOT NULL,
    execution_context TEXT,
    last_updated DATETIME NOT NULL,
    version INTEGER NOT NULL,
    FOREIGN KEY (job_execution_id) REFERENCES batch_job_execution(id)
);

CREATE INDEX IF NOT EXISTS idx_job_execution_id ON batch_step_execution (job_execution_id);
