// Package repository provides interfaces and implementations for weather data persistence.
package repository

import (
	"context"
	_ "database/sql"
	"fmt"

	"github.com/tigerroll/surfin/pkg/batch/adapter/database"
	tx "github.com/tigerroll/surfin/pkg/batch/core/tx"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"

	weather_entity "github.com/tigerroll/surfin/example/weather/internal/domain/entity"
)

// WeatherRepository defines the interface for storing and managing weather data.
type WeatherRepository interface {
	// BulkInsertWeatherData performs a bulk insert of weather data.
	// It takes a transaction (tx.Tx) to ensure atomicity.
	BulkInsertWeatherData(ctx context.Context, tx tx.Tx, items []weather_entity.WeatherDataToStore) error
	// TruncateHourlyForecast deletes all data from the hourly_forecast table.
	TruncateHourlyForecast(ctx context.Context) error
	Close() error
}

// PostgresRepositoryWrapper adapts repository.PostgresRepository to WeatherRepository.
type PostgresRepositoryWrapper struct {
	dbConn database.DBConnection
	dbType string // The type of the database (e.g., "postgres").
}

// NewPostgresWeatherRepository creates a new instance of PostgresRepositoryWrapper.
//
// Parameters:
//   - dbConn: The database connection to use.
//   - dbType: The type of the database (e.g., "postgres").
//
// Returns:
//   - An implementation of the WeatherRepository interface for PostgreSQL.
func NewPostgresWeatherRepository(dbConn database.DBConnection, dbType string) WeatherRepository {
	return &PostgresRepositoryWrapper{dbConn: dbConn, dbType: dbType}
}

func (w *PostgresRepositoryWrapper) BulkInsertWeatherData(ctx context.Context, tx tx.Tx, items []weather_entity.WeatherDataToStore) error {
	if len(items) == 0 {
		return nil
	}
	logger.Debugf("PostgresRepositoryWrapper.BulkInsertWeatherData: Using DB Type: %s. Attempting bulk insert via Tx.", w.dbType)

	// Use ExecuteUpsert to perform ON CONFLICT DO NOTHING
	conflictCols := []string{"time", "latitude", "longitude"}
	updateCols := []string{} // DO NOTHING

	// Get table name using items[0].TableName()
	rowsAffected, err := tx.ExecuteUpsert(ctx, &items, items[0].TableName(), conflictCols, updateCols)

	if err != nil {
		return fmt.Errorf("failed to bulk insert hourly_forecast data using ExecuteUpsert: %w", err)
	}

	// RowsAffected might be 0 when using ON CONFLICT DO NOTHING. The insertion attempt is considered successful if no error occurs.
	logger.Debugf("PostgresRepositoryWrapper: Attempted to write %d weather data items to hourly_forecast (RowsAffected: %d).", len(items), rowsAffected)
	return nil
}

// TruncateHourlyForecast deletes all data from the hourly_forecast table.
func (w *PostgresRepositoryWrapper) TruncateHourlyForecast(ctx context.Context) error {
	const op = "PostgresRepositoryWrapper.TruncateHourlyForecast"

	// Use ExecuteUpdate for DELETE operation
	dummyEntity := weather_entity.WeatherDataToStore{}

	// DELETE FROM hourly_forecast.
	// ExecuteUpdate generates a DELETE query that deletes the entire table if no WHERE clause is provided.
	// Explicitly pass the table name.
	_, err := w.dbConn.ExecuteUpdate(ctx, &dummyEntity, "DELETE", dummyEntity.TableName(), nil)

	if err != nil {
		return exception.NewBatchError(op, "Table data deletion error", err, false, false)
	}
	logger.Infof("%s: Data deletion from hourly_forecast table completed.", op)
	return nil
}

func (w *PostgresRepositoryWrapper) Close() error {
	// The underlying DBConnection is managed by the framework; therefore, it should not be closed here.
	return nil
}

// MySQLRepositoryWrapper adapts repository.MySQLRepository to WeatherRepository.
type MySQLRepositoryWrapper struct {
	dbConn database.DBConnection
	dbType string // The type of the database (e.g., "mysql").
}

// NewMySQLWeatherRepository creates a new instance of MySQLRepositoryWrapper.
//
// Parameters:
//   - dbConn: The database connection to use.
//   - dbType: The type of the database (e.g., "mysql").
//
// Returns:
//   - An implementation of the WeatherRepository interface for MySQL.
func NewMySQLWeatherRepository(dbConn database.DBConnection, dbType string) WeatherRepository {
	return &MySQLRepositoryWrapper{dbConn: dbConn, dbType: dbType}
}

// BulkInsertWeatherData performs a bulk insert of weather data into MySQL.
// It uses ON DUPLICATE KEY UPDATE for existing records.
// It takes a transaction (tx.Tx) to ensure atomicity.
func (w *MySQLRepositoryWrapper) BulkInsertWeatherData(ctx context.Context, tx tx.Tx, items []weather_entity.WeatherDataToStore) error {
	if len(items) == 0 {
		return nil
	}
	logger.Debugf("MySQLRepositoryWrapper.BulkInsertWeatherData: Using DB Type: %s. Attempting Bulk Insert via Tx.", w.dbType)

	// Use ExecuteUpsert to perform ON DUPLICATE KEY UPDATE
	conflictCols := []string{"time", "latitude", "longitude"}
	updateCols := []string{"weather_code", "temperature_2m", "collected_at"} // DO UPDATE

	// Get table name using items[0].TableName()
	rowsAffected, err := tx.ExecuteUpsert(ctx, &items, items[0].TableName(), conflictCols, updateCols)

	if err != nil {
		return fmt.Errorf("failed to bulk insert hourly_forecast data using ExecuteUpsert: %w", err)
	}

	// RowsAffected behavior can be complex with ON DUPLICATE KEY UPDATE; logging the attempted item count.
	logger.Debugf("MySQLRepositoryWrapper: Attempted to write %d weather data items to hourly_forecast (RowsAffected: %d).", len(items), rowsAffected)
	return nil
}

// TruncateHourlyForecast deletes all data from the hourly_forecast table.
func (w *MySQLRepositoryWrapper) TruncateHourlyForecast(ctx context.Context) error {
	const op = "MySQLRepositoryWrapper.TruncateHourlyForecast"

	// Use ExecuteUpdate for DELETE operation
	dummyEntity := weather_entity.WeatherDataToStore{}

	// DELETE FROM hourly_forecast.
	// Explicitly pass the table name.
	_, err := w.dbConn.ExecuteUpdate(ctx, &dummyEntity, "DELETE", dummyEntity.TableName(), nil)

	if err != nil {
		return exception.NewBatchError(op, "Table data deletion error", err, false, false)
	}
	logger.Infof("%s: Data deletion from hourly_forecast table completed.", op)
	return nil
}

func (w *MySQLRepositoryWrapper) Close() error {
	// The underlying DBConnection is managed by the framework; therefore, it should not be closed here.
	return nil
}

// SQLiteRepositoryWrapper adapts repository.SQLiteRepository to WeatherRepository.
type SQLiteRepositoryWrapper struct {
	dbConn database.DBConnection
	dbType string // The type of the database (e.g., "sqlite").
}

// NewSQLiteWeatherRepository creates a new instance of SQLiteRepositoryWrapper.
//
// Parameters:
//   - dbConn: The database connection to use.
//   - dbType: The type of the database (e.g., "sqlite").
//
// Returns:
//   - An implementation of the WeatherRepository interface for SQLite.
func NewSQLiteWeatherRepository(dbConn database.DBConnection, dbType string) WeatherRepository {
	return &SQLiteRepositoryWrapper{dbConn: dbConn, dbType: dbType}
}

// BulkInsertWeatherData performs a bulk insert of weather data into SQLite.
// It uses ON CONFLICT DO NOTHING for existing records.
// It takes a transaction (tx.Tx) to ensure atomicity.
func (w *SQLiteRepositoryWrapper) BulkInsertWeatherData(ctx context.Context, tx tx.Tx, items []weather_entity.WeatherDataToStore) error {
	if len(items) == 0 {
		return nil
	}
	logger.Debugf("SQLiteRepositoryWrapper.BulkInsertWeatherData: Using DB Type: %s. Attempting Bulk Insert via Tx.", w.dbType)

	// Use ExecuteUpsert to perform ON CONFLICT DO NOTHING
	conflictCols := []string{"time", "latitude", "longitude"}
	updateCols := []string{} // DO NOTHING

	// Get table name using items[0].TableName()
	rowsAffected, err := tx.ExecuteUpsert(ctx, &items, items[0].TableName(), conflictCols, updateCols)

	if err != nil {
		return fmt.Errorf("failed to bulk insert hourly_forecast data into SQLite using ExecuteUpsert: %w", err)
	}

	logger.Debugf("SQLiteRepositoryWrapper: Attempted to write %d weather data items to hourly_forecast (RowsAffected: %d).", len(items), rowsAffected)
	return nil
}

// TruncateHourlyForecast deletes all data from the hourly_forecast table in SQLite.
func (w *SQLiteRepositoryWrapper) TruncateHourlyForecast(ctx context.Context) error {
	// Define the operation name for logging and error reporting.
	const op = "SQLiteRepositoryWrapper.TruncateHourlyForecast"

	// Use ExecuteUpdate for DELETE operation
	dummyEntity := weather_entity.WeatherDataToStore{}

	// DELETE FROM hourly_forecast
	// Explicitly pass the table name.
	_, err := w.dbConn.ExecuteUpdate(ctx, &dummyEntity, "DELETE", dummyEntity.TableName(), nil)

	if err != nil {
		return exception.NewBatchError(op, "Table data deletion error", err, false, false)
	}
	logger.Infof("%s: Data deletion from hourly_forecast table completed.", op)
	return nil
}

func (w *SQLiteRepositoryWrapper) Close() error {
	// The underlying DBConnection is managed by the framework; therefore, it should not be closed here.
	return nil
}
