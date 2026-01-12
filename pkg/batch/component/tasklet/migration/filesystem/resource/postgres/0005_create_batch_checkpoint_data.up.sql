CREATE TABLE IF NOT EXISTS batch_checkpoint_data (
    step_execution_id VARCHAR(36) PRIMARY KEY REFERENCES batch_step_execution(id),
    execution_context TEXT,
    last_updated TIMESTAMP WITH TIME ZONE NOT NULL
);
