-- Revert initial framework schema for PostgreSQL.
SET search_path TO batch_metadata;

DROP SCHEMA IF EXISTS batch_metadata;
