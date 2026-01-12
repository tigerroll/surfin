CREATE TABLE IF NOT EXISTS batch_job_execution (
    id VARCHAR(36) PRIMARY KEY,
    job_instance_id VARCHAR(36) NOT NULL REFERENCES batch_job_instance(id),
    job_name VARCHAR(255) NOT NULL,
    parameters TEXT NOT NULL,
    start_time TIMESTAMP WITH TIME ZONE NOT NULL,
    end_time TIMESTAMP WITH TIME ZONE,
    status VARCHAR(30) NOT NULL,
    exit_status VARCHAR(30) NOT NULL,
    exit_code INTEGER NOT NULL,
    failures TEXT,
    version INTEGER NOT NULL DEFAULT 0,
    create_time TIMESTAMP WITH TIME ZONE NOT NULL,
    last_updated TIMESTAMP WITH TIME ZONE NOT NULL,
    execution_context TEXT,
    current_step_name VARCHAR(255),
    restart_count INTEGER NOT NULL DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_job_instance_id ON batch_job_execution (job_instance_id);
