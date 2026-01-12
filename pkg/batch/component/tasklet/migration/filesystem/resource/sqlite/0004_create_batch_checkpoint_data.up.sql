CREATE TABLE IF NOT EXISTS batch_checkpoint_data (
    step_execution_id TEXT PRIMARY KEY,
    execution_context TEXT,
    last_updated DATETIME NOT NULL,
    FOREIGN KEY (step_execution_id) REFERENCES batch_step_execution(id)
);
