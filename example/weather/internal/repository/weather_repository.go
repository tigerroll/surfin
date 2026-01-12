package repository

import (
	"context"
	_ "database/sql"
	"fmt"

	"surfin/pkg/batch/core/adaptor"
	tx "surfin/pkg/batch/core/tx"
	"surfin/pkg/batch/support/util/exception"
	"surfin/pkg/batch/support/util/logger"

	weather_entity "surfin/example/weather/internal/domain/entity"
)

type WeatherRepository interface {
	// BulkInsertWeatherData passes the transaction.
	BulkInsertWeatherData(ctx context.Context, tx tx.Tx, items []weather_entity.WeatherDataToStore) error
	// TruncateHourlyForecast deletes all data from the hourly_forecast table.
	TruncateHourlyForecast(ctx context.Context) error
	Close() error
}

// PostgresRepositoryWrapper adapts repository.PostgresRepository to WeatherRepository.
type PostgresRepositoryWrapper struct {
	dbConn adaptor.DBConnection
	dbType string
}

func NewPostgresWeatherRepository(dbConn adaptor.DBConnection, dbType string) WeatherRepository {
	return &PostgresRepositoryWrapper{dbConn: dbConn, dbType: dbType}
}

func (w *PostgresRepositoryWrapper) BulkInsertWeatherData(ctx context.Context, tx tx.Tx, items []weather_entity.WeatherDataToStore) error {
	if len(items) == 0 {
		return nil
	}
	logger.Debugf("PostgresRepositoryWrapper.BulkInsertWeatherData: Using DB Type: %s. Attempting Bulk Insert via Tx.", w.dbType)

	// ExecuteUpsert を使用して ON CONFLICT DO NOTHING を実行
	conflictCols := []string{"time", "latitude", "longitude"}
	updateCols := []string{} // DO NOTHING

	// items[0].TableName() でテーブル名を取得
	rowsAffected, err := tx.ExecuteUpsert(ctx, &items, items[0].TableName(), conflictCols, updateCols)

	if err != nil {
		return fmt.Errorf("failed to bulk insert hourly_forecast data using ExecuteUpsert: %w", err)
	}

	// RowsAffected might be 0 when using ON CONFLICT DO NOTHING. Insertion attempt is successful if no error occurs.
	logger.Debugf("PostgresRepositoryWrapper: Attempted to write %d weather data items to hourly_forecast (RowsAffected: %d).", len(items), rowsAffected)
	return nil
}

// TruncateHourlyForecast deletes all data from the hourly_forecast table.
func (w *PostgresRepositoryWrapper) TruncateHourlyForecast(ctx context.Context) error {
	const op = "PostgresRepositoryWrapper.TruncateHourlyForecast"
	
	// T_GORM_2: ExecuteUpdate の DELETE 操作を使用
	dummyEntity := weather_entity.WeatherDataToStore{}
	
	// DELETE FROM hourly_forecast
	// ExecuteUpdate は DELETE 操作の場合、WHERE句がなくてもテーブル全体を削除する DELETE クエリを生成する
	// テーブル名を明示的に渡す
	_, err := w.dbConn.ExecuteUpdate(ctx, &dummyEntity, "DELETE", dummyEntity.TableName(), nil)

	if err != nil {
		return exception.NewBatchError(op, "Table data deletion error", err, false, false)
	}
	logger.Infof("%s: Data deletion from hourly_forecast table completed.", op)
	return nil
}

func (w *PostgresRepositoryWrapper) Close() error {
	// The underlying DBConnection is managed by the framework, so do not close here.
	return nil
}

// MySQLRepositoryWrapper adapts repository.MySQLRepository to WeatherRepository.
type MySQLRepositoryWrapper struct {
	dbConn adaptor.DBConnection
	dbType string
}

func NewMySQLWeatherRepository(dbConn adaptor.DBConnection, dbType string) WeatherRepository {
	return &MySQLRepositoryWrapper{dbConn: dbConn, dbType: dbType}
}

func (w *MySQLRepositoryWrapper) BulkInsertWeatherData(ctx context.Context, tx tx.Tx, items []weather_entity.WeatherDataToStore) error {
	if len(items) == 0 {
		return nil
	}
	logger.Debugf("MySQLRepositoryWrapper.BulkInsertWeatherData: Using DB Type: %s. Attempting Bulk Insert via Tx.", w.dbType)

	// ExecuteUpsert を使用して ON DUPLICATE KEY UPDATE を実行
	conflictCols := []string{"time", "latitude", "longitude"}
	updateCols := []string{"weather_code", "temperature_2m", "collected_at"} // DO UPDATE

	// items[0].TableName() でテーブル名を取得
	rowsAffected, err := tx.ExecuteUpsert(ctx, &items, items[0].TableName(), conflictCols, updateCols)

	if err != nil {
		return fmt.Errorf("failed to bulk insert hourly_forecast data using ExecuteUpsert: %w", err)
	}

	// RowsAffected behavior is complex with ON DUPLICATE KEY UPDATE; logging attempted item count.
	logger.Debugf("MySQLRepositoryWrapper: Attempted to write %d weather data items to hourly_forecast (RowsAffected: %d).", len(items), rowsAffected)
	return nil
}

// TruncateHourlyForecast deletes all data from the hourly_forecast table.
func (w *MySQLRepositoryWrapper) TruncateHourlyForecast(ctx context.Context) error {
	const op = "MySQLRepositoryWrapper.TruncateHourlyForecast"
	
	// T_GORM_2: ExecuteUpdate の DELETE 操作を使用
	dummyEntity := weather_entity.WeatherDataToStore{}
	
	// DELETE FROM hourly_forecast
	// テーブル名を明示的に渡す
	_, err := w.dbConn.ExecuteUpdate(ctx, &dummyEntity, "DELETE", dummyEntity.TableName(), nil)

	if err != nil {
		return exception.NewBatchError(op, "Table data deletion error", err, false, false)
	}
	logger.Infof("%s: Data deletion from hourly_forecast table completed.", op)
	return nil
}

func (w *MySQLRepositoryWrapper) Close() error {
	// The underlying DBConnection is managed by the framework, so do not close here.
	return nil
}

// SQLiteRepositoryWrapper adapts repository.SQLiteRepository to WeatherRepository.
type SQLiteRepositoryWrapper struct {
	dbConn adaptor.DBConnection
	dbType string
}

func NewSQLiteWeatherRepository(dbConn adaptor.DBConnection, dbType string) WeatherRepository {
	return &SQLiteRepositoryWrapper{dbConn: dbConn, dbType: dbType}
}

func (w *SQLiteRepositoryWrapper) BulkInsertWeatherData(ctx context.Context, tx tx.Tx, items []weather_entity.WeatherDataToStore) error {
	if len(items) == 0 {
		return nil
	}
	logger.Debugf("SQLiteRepositoryWrapper.BulkInsertWeatherData: Using DB Type: %s. Attempting Bulk Insert via Tx.", w.dbType)

	// ExecuteUpsert を使用して ON CONFLICT DO NOTHING を実行
	conflictCols := []string{"time", "latitude", "longitude"}
	updateCols := []string{} // DO NOTHING

	// items[0].TableName() でテーブル名を取得
	rowsAffected, err := tx.ExecuteUpsert(ctx, &items, items[0].TableName(), conflictCols, updateCols)

	if err != nil {
		return fmt.Errorf("failed to bulk insert hourly_forecast data into SQLite using ExecuteUpsert: %w", err)
	}

	logger.Debugf("SQLiteRepositoryWrapper: Attempted to write %d weather data items to hourly_forecast (RowsAffected: %d).", len(items), rowsAffected)
	return nil
}

func (w *SQLiteRepositoryWrapper) TruncateHourlyForecast(ctx context.Context) error {
	const op = "SQLiteRepositoryWrapper.TruncateHourlyForecast"
	
	// T_GORM_2: ExecuteUpdate の DELETE 操作を使用
	dummyEntity := weather_entity.WeatherDataToStore{}
	
	// DELETE FROM hourly_forecast
	// テーブル名を明示的に渡す
	_, err := w.dbConn.ExecuteUpdate(ctx, &dummyEntity, "DELETE", dummyEntity.TableName(), nil)

	if err != nil {
		return exception.NewBatchError(op, "Table data deletion error", err, false, false)
	}
	logger.Infof("%s: Data deletion from hourly_forecast table completed.", op)
	return nil
}

func (w *SQLiteRepositoryWrapper) Close() error {
	// The underlying DBConnection is managed by the framework, so do not close here.
	return nil
}
