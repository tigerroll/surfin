// Package dummy provides dummy implementations for database-related interfaces.
// These implementations are intended for use in DB-less environments or for testing purposes,
// where actual database operations are not required.
package dummy

import (
	"context"

	"go.uber.org/fx"

	"github.com/tigerroll/surfin/pkg/batch/core/adaptor"
	"github.com/tigerroll/surfin/pkg/batch/core/config"
	"github.com/tigerroll/surfin/pkg/batch/core/tx"
	"github.com/tigerroll/surfin/pkg/batch/support/util/logger"
)

func NewDummyDBConnectionsAndTxManagers(p struct {
	fx.In
	Lifecycle fx.Lifecycle
	Cfg       *config.Config
}) (
	map[string]adaptor.DBConnection,
	map[string]tx.TransactionManager,
	map[string]adaptor.DBProvider,
	tx.TransactionManagerFactory,
	error,
) {
	// NewDummyDBConnectionsAndTxManagers provides dummy implementations for DB connections and transaction managers.
	// These are used when the application runs in DB-less mode.
	logger.Warnf("Running in DB-less mode. Providing dummy DB connections and transaction managers.")

	// Add a hook to the Fx lifecycle to prevent errors from nil providers during shutdown.
	p.Lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.Infof("Dummy DB connections and transaction managers are being closed.")
			return nil
		},
	})

	// Provide a dummy DBProvider that can be resolved by type "dummy".
	dummyProviders := make(map[string]adaptor.DBProvider)
	dummyProviders["dummy"] = &dummyDBProvider{}
	return make(map[string]adaptor.DBConnection), make(map[string]tx.TransactionManager), make(map[string]adaptor.DBProvider), &dummyTxManagerFactory{}, nil
}

// NewMetadataTxManager extracts the "metadata" TransactionManager from the map.
// In DB-less mode, it always returns a dummy manager provided by the dummyFactory.
func NewMetadataTxManager(allTxManagers map[string]tx.TransactionManager, dummyFactory tx.TransactionManagerFactory) (tx.TransactionManager, error) {
	return dummyFactory.NewTransactionManager(nil), nil // A nil connection is passed as it's a dummy.
}

// Module is an Fx module that provides dummy DB-related implementations.
var Module = fx.Options(
	fx.Provide(NewDummyDBConnectionsAndTxManagers),
	fx.Provide(fx.Annotate(
		NewMetadataTxManager,
		fx.ResultTags(`name:"metadata"`),
	)),
	fx.Provide(fx.Annotate(
		NewDefaultDBConnectionResolver,
		fx.As(new(adaptor.DBConnectionResolver)),
	)),
)
