CREATE TABLE IF NOT EXISTS batch_checkpoint_data (
    step_execution_id VARCHAR(36) PRIMARY KEY,
    execution_context TEXT NULL,
    last_updated DATETIME NOT NULL,
    FOREIGN KEY (step_execution_id) REFERENCES batch_step_execution(id)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4;
