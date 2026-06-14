package writer

import (
	"context"
	"fmt"

	weather_entity "github.com/tigerroll/surfin/example/weather/internal/domain/entity"
	"github.com/tigerroll/surfin/pkg/batch/adapter/database"
	"github.com/tigerroll/surfin/pkg/batch/component/step/writer"
	port "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	batch_config "github.com/tigerroll/surfin/pkg/batch/core/config"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/support/util/configbinder"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
)

type HourlyHistoryDatabaseWriterConfig struct {
	TargetResourceName string `yaml:"targetResourceName,omitempty"`
	Database           string `yaml:"database,omitempty"`
	BulkSize           int    `yaml:"bulkSize,omitempty"`
}

type HourlyHistoryDatabaseWriter struct {
	sqlBulkWriter *writer.SqlBulkWriter[weather_entity.HourlyHistoryToStore]

	DBResolver         database.DBConnectionResolver
	Config             *batch_config.Config
	TargetResourceName string
	ResourcePath       string
	bulkSize           int

	stepExecutionContext model.ExecutionContext
	writerState          model.ExecutionContext
	resolver             port.ExpressionResolver
}

var _ port.ItemWriter[any] = (*HourlyHistoryDatabaseWriter)(nil)

func NewHourlyHistoryDatabaseWriter(
	cfg *batch_config.Config,
	resolver port.ExpressionResolver,
	dbResolver database.DBConnectionResolver,
	properties map[string]interface{},
) (*HourlyHistoryDatabaseWriter, error) {

	writerCfg := &HourlyHistoryDatabaseWriterConfig{}
	if err := configbinder.BindProperties(properties, writerCfg); err != nil {
		return nil, exception.NewBatchError("hourly_history_database_writer", "Failed to bind properties", err, false, false)
	}

	resourceName := writerCfg.TargetResourceName
	if resourceName == "" {
		resourceName = writerCfg.Database
	}
	if resourceName == "" {
		resourceName = "workload"
	}

	bulkSize := writerCfg.BulkSize
	if bulkSize <= 0 {
		bulkSize = 1000
	}

	return &HourlyHistoryDatabaseWriter{
		DBResolver:         dbResolver,
		Config:             cfg,
		TargetResourceName: resourceName,
		ResourcePath:       weather_entity.HourlyHistoryToStore{}.TableName(),
		bulkSize:           bulkSize,

		resolver:             resolver,
		stepExecutionContext: model.NewExecutionContext(),
		writerState:          model.NewExecutionContext(),
	}, nil
}

func (w *HourlyHistoryDatabaseWriter) Open(ctx context.Context, ec model.ExecutionContext) error {
	_, err := w.DBResolver.ResolveDBConnection(ctx, w.TargetResourceName)
	if err != nil {
		return fmt.Errorf("failed to resolve database connection '%s': %w", w.TargetResourceName, err)
	}

	w.sqlBulkWriter = writer.NewSqlBulkWriter[weather_entity.HourlyHistoryToStore](
		"hourly_history_writer",
		w.bulkSize,
		w.ResourcePath,
		[]string{"time", "latitude", "longitude"},
		[]string{"weather_code", "temperature_2m", "collected_at"},
	)

	if err := w.sqlBulkWriter.Open(ctx, ec); err != nil {
		return fmt.Errorf("failed to open SqlBulkWriter: %w", err)
	}

	w.stepExecutionContext = ec
	return nil
}

func (w *HourlyHistoryDatabaseWriter) Write(ctx context.Context, items []any) error {
	if len(items) == 0 {
		return nil
	}

	var finalDataToStore []weather_entity.HourlyHistoryToStore
	for _, item := range items {
		typedItem, ok := item.(*weather_entity.HourlyHistoryToStore)
		if !ok {
			return exception.NewBatchError("hourly_history_database_writer", fmt.Sprintf("unexpected input item type: %T", item), nil, false, true)
		}
		if typedItem != nil {
			finalDataToStore = append(finalDataToStore, *typedItem)
		}
	}

	if len(finalDataToStore) == 0 {
		return nil
	}

	return w.sqlBulkWriter.Write(ctx, finalDataToStore)
}

func (w *HourlyHistoryDatabaseWriter) Close(ctx context.Context) error {
	if w.sqlBulkWriter != nil {
		return w.sqlBulkWriter.Close(ctx)
	}
	return nil
}

func (w *HourlyHistoryDatabaseWriter) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	w.stepExecutionContext = ec
	return nil
}

func (w *HourlyHistoryDatabaseWriter) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	return w.writerState, nil
}

func (w *HourlyHistoryDatabaseWriter) GetResourcePath() string {
	return w.ResourcePath
}

func (w *HourlyHistoryDatabaseWriter) GetTargetResourceName() string {
	return w.TargetResourceName
}
