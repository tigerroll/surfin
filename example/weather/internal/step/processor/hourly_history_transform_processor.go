// Package processor provides implementations of ItemProcessor for transforming weather data.
package processor

import (
	"context"
	"fmt"

	weather_entity "github.com/tigerroll/surfin/example/weather/internal/domain/entity"
	core "github.com/tigerroll/surfin/pkg/batch/core/application/port"
	config "github.com/tigerroll/surfin/pkg/batch/core/config"
	model "github.com/tigerroll/surfin/pkg/batch/core/domain/model"
	"github.com/tigerroll/surfin/pkg/batch/support/util/exception"
)

// HourlyHistoryTransformProcessor validates and passes through historical weather data.
// Since the Reader already performs the transformation, this processor acts as a pass-through.
type HourlyHistoryTransformProcessor struct{}

// NewHourlyHistoryTransformProcessor creates a new instance of HourlyHistoryTransformProcessor.
func NewHourlyHistoryTransformProcessor(cfg *config.Config, resolver core.ExpressionResolver, properties map[string]interface{}) (*HourlyHistoryTransformProcessor, error) {
	return &HourlyHistoryTransformProcessor{}, nil
}

// Process validates the input item type and passes it through to the writer.
func (p *HourlyHistoryTransformProcessor) Process(ctx context.Context, item any) (any, error) {
	select {
	case <-ctx.Done():
		return nil, ctx.Err()
	default:
	}

	// The Reader already returns *weather_entity.HourlyHistoryToStore, so we pass it through.
	if itemToStore, ok := item.(*weather_entity.HourlyHistoryToStore); ok {
		return itemToStore, nil
	}

	return nil, exception.NewBatchError("hourly_history_transform_processor", fmt.Sprintf("unexpected input item type: %T", item), nil, false, true)
}

// SetExecutionContext sets the execution context for the processor.
func (p *HourlyHistoryTransformProcessor) SetExecutionContext(ctx context.Context, ec model.ExecutionContext) error {
	return nil
}

// GetExecutionContext retrieves the current execution context.
func (p *HourlyHistoryTransformProcessor) GetExecutionContext(ctx context.Context) (model.ExecutionContext, error) {
	return nil, nil
}

var _ core.ItemProcessor[any, any] = (*HourlyHistoryTransformProcessor)(nil)
