CREATE TABLE IF NOT EXISTS batch_step_execution (
    id VARCHAR(36) PRIMARY KEY,
    step_name VARCHAR(255) NOT NULL,
    job_execution_id VARCHAR(36) NOT NULL REFERENCES batch_job_execution(id),
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP WITH TIME ZONE,
    status VARCHAR(30) NOT NULL,
    exit_status VARCHAR(30) NOT NULL,
    failures TEXT,
    read_count INTEGER NOT NULL DEFAULT 0,
    write_count INTEGER NOT NULL DEFAULT 0,
    commit_count INTEGER NOT NULL DEFAULT 0,
    rollback_count INTEGER NOT NULL DEFAULT 0,
    filter_count INTEGER NOT NULL DEFAULT 0,
    skip_read_count INTEGER NOT NULL DEFAULT 0,
    skip_process_count INTEGER NOT NULL DEFAULT 0,
    skip_write_count INTEGER NOT NULL DEFAULT 0,
    execution_context TEXT,
    last_updated TIMESTAMP WITH TIME ZONE NOT NULL,
    version INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_job_execution_id ON batch_step_execution (job_execution_id);
