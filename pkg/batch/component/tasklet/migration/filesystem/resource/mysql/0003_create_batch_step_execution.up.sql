CREATE TABLE IF NOT EXISTS batch_step_execution (
    id VARCHAR(36) PRIMARY KEY,
    step_name VARCHAR(255) NOT NULL,
    job_execution_id VARCHAR(36) NOT NULL,
    start_time DATETIME NOT NULL,
    end_time DATETIME NULL,
    status VARCHAR(30) NOT NULL,
    exit_status VARCHAR(30) NOT NULL,
    failures TEXT NULL,
    read_count INT NOT NULL,
    write_count INT NOT NULL,
    commit_count INT NOT NULL,
    rollback_count INT NOT NULL,
    filter_count INT NOT NULL,
    skip_read_count INT NOT NULL,
    skip_process_count INT NOT NULL,
    skip_write_count INT NOT NULL,
    execution_context TEXT NULL,
    last_updated DATETIME NOT NULL,
    version INT NOT NULL,
    INDEX idx_job_execution_id (job_execution_id),
    FOREIGN KEY (job_execution_id) REFERENCES batch_job_execution(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;


