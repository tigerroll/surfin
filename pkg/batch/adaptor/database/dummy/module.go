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

// NewDummyDBConnectionsAndTxManagers provides dummy implementations for database connections and transaction managers.
// These are used when the application is configured to run in a DB-less mode, typically for testing or specific operational scenarios where actual database interaction is not required.
//
// Parameters:
//   p: An Fx parameter struct containing Lifecycle and Config.
//
// Returns:
//   A map of dummy DBConnection instances, a map of dummy TransactionManager instances,
//   a map of dummy DBProvider instances, a dummy TransactionManagerFactory, and an error.
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
	return make(map[string]adaptor.DBConnection), make(map[string]tx.TransactionManager), make(map[string]adaptor.DBProvider), &DummyTxManagerFactory{}, nil
}

// NewMetadataTxManager provides a dummy TransactionManager specifically for "metadata" operations.
// In DB-less mode, it always returns a dummy manager created by the provided dummyFactory,
// effectively bypassing actual database interactions for metadata.
//
// Parameters:
//   allTxManagers: A map of all registered TransactionManagers (ignored in dummy mode).
//   dummyFactory: The dummy TransactionManagerFactory to create the dummy manager.
//
// Returns:
//   A dummy TransactionManager and a nil error.
func NewMetadataTxManager(allTxManagers map[string]tx.TransactionManager, dummyFactory tx.TransactionManagerFactory) (tx.TransactionManager, error) {
	return dummyFactory.NewTransactionManager(nil), nil // A nil connection is passed as it's a dummy.
}

// Module is an Fx module that provides dummy database-related implementations.
// This module is intentionally empty in its current state, as the dummy providers
// are conditionally supplied by the main application setup when DB-less mode is detected.
var Module = fx.Options(
	// fx.Provide(NewDummyDBConnectionsAndTxManagers), // This provider is commented out as it's conditionally provided elsewhere.
)
