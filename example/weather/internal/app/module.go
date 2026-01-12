package app

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sync"
	// "time" // 削除

	"surfin/pkg/batch/core/adaptor"
	config "surfin/pkg/batch/core/config"
	tx "surfin/pkg/batch/core/tx" // 修正: 新しいパスを使用
	"surfin/pkg/batch/support/util/logger"

	gormadaptor "surfin/pkg/batch/adaptor/database/gorm" // パッケージ名を gormadaptor としてインポート
	"surfin/pkg/batch/adaptor/database/gorm/mysql"
	"surfin/pkg/batch/adaptor/database/gorm/postgres"
	"surfin/pkg/batch/adaptor/database/gorm/sqlite"
	migrationfs "surfin/pkg/batch/component/tasklet/migration/filesystem"
	
	// surfin/pkg/batch/core/configuration/bootstrap のインポートは不要になりました

	"go.uber.org/fx"
)

// DBProviderMap は、main.go が動的にプロバイダを選択するために使用されます。
var DBProviderMap = map[string]func(cfg *config.Config) adaptor.DBProvider {
	"postgres": postgres.NewProvider,
	"redshift": postgres.NewProvider, // Redshift も PostgresProvider を使用
	"mysql":    mysql.NewProvider,
	"sqlite":   sqlite.NewProvider,
}

// MigrationFSMapParams defines the dependencies for NewMigrationFSMap.
type MigrationFSMapParams struct {
	fx.In
	WeatherAppFS fs.FS `name:"weatherAppFS"` // Provided by the anonymous provider below
	FrameworkFS  fs.FS `name:"frameworkMigrationsFS"` // Provided by migrationfs.Module
}

// NewMigrationFSMap aggregates all necessary migration file systems into a single map.
func NewMigrationFSMap(p MigrationFSMapParams) map[string]fs.FS {
	fsMap := make(map[string]fs.FS)

	// 1. Add Framework FS
	// Framework FS is provided with the name "frameworkMigrationsFS"
	frameworkFSName := "frameworkMigrationsFS" 
	if p.FrameworkFS != nil {
		fsMap[frameworkFSName] = p.FrameworkFS 
	}

	// 2. Add Application FS
	if p.WeatherAppFS != nil {
		fsMap["weatherAppFS"] = p.WeatherAppFS
	}

	logger.Debugf("Aggregated %d total migration FSs into a map.", len(fsMap))
	return fsMap
}

// DBConnectionsAndTxManagersParams defines the dependencies for NewDBConnectionsAndTxManagers.
type DBConnectionsAndTxManagersParams struct {
	fx.In
	Lifecycle fx.Lifecycle
	Cfg       *config.Config
	// Fx automatically collects all components tagged with adaptor.DBProviderGroup into a slice.
	DBProviders []adaptor.DBProvider `group:"db_providers"`
	// TransactionManagerFactory を注入
	TxFactory tx.TransactionManagerFactory
}

// NewDBConnectionsAndTxManagers は、設定ファイルに定義されたすべてのデータソースに対して、
// 適切な DBProvider を使用して接続とトランザクションマネージャを確立し、マップとして提供します。
func NewDBConnectionsAndTxManagers(p DBConnectionsAndTxManagersParams) (
	map[string]adaptor.DBConnection,
	map[string]tx.TransactionManager,
	map[string]adaptor.DBProvider, // DBProviderのマップも提供する
	error,
) {
	allConnections := make(map[string]adaptor.DBConnection)
	allTxManagers := make(map[string]tx.TransactionManager)
	allProviders := make(map[string]adaptor.DBProvider)
	
	// ProviderをDBタイプでマップする
	providerMap := make(map[string]adaptor.DBProvider)
	for _, provider := range p.DBProviders {
		providerMap[provider.Type()] = provider
		allProviders[provider.Type()] = provider // DBタイプでプロバイダを保存
	}

	// 設定ファイルに定義されたすべてのデータソースをループ処理
	for name, dbConfig := range p.Cfg.Surfin.Datasources {
		provider, ok := providerMap[dbConfig.Type]
		if !ok {
			// PostgresProviderはRedshiftも扱うため、ここでは厳密なチェックを避ける
			if dbConfig.Type == "redshift" {
				provider, ok = providerMap["postgres"]
			}
			if !ok {
				logger.Warnf("No DBProvider found for database type '%s' (Datasource: %s). Skipping connection.", dbConfig.Type, name)
				continue
			}
		}

		// Providerを使用して接続を取得
		conn, err := provider.GetConnection(name)
		if err != nil {
			return nil, nil, nil, fmt.Errorf("failed to get connection for '%s' using provider '%s': %w", name, provider.Type(), err)
		}
		
		// 接続が確立されたら、TxManagerを作成
		// 抽象ファクトリを使用して TxManager を作成する
		txManager := p.TxFactory.NewTransactionManager(conn)
		
		allConnections[name] = conn
		allTxManagers[name] = txManager
		logger.Debugf("Initialized DB Connection and TxManager for: %s (%s)", name, dbConfig.Type)
	}

	// Fxライフサイクルにフックを追加し、シャットダウン時にすべての接続を閉じる
	p.Lifecycle.Append(fx.Hook{
		OnStop: func(ctx context.Context) error {
			logger.Infof("Closing all database connections...")
			var wg sync.WaitGroup
			var lastErr error
			
			// プロバイダごとに接続を閉じる
			for _, provider := range p.DBProviders {
				wg.Add(1)
				go func(p adaptor.DBProvider) {
					defer wg.Done()
					// 接続のクローズはプロバイダに委譲
					if err := p.CloseAll(); err != nil {
						logger.Errorf("Failed to close connections for provider %s: %v", p.Type(), err)
						lastErr = err
					}
				}(provider)
			}
			wg.Wait()
			return lastErr
		},
	})

	return allConnections, allTxManagers, allProviders, nil
}

// NewMetadataTxManager extracts the "metadata" TransactionManager from the map.
func NewMetadataTxManager(allTxManagers map[string]tx.TransactionManager) (tx.TransactionManager, error) {
	tm, ok := allTxManagers["metadata"]
	if !ok {
		return nil, fmt.Errorf("metadata TransactionManager not found in aggregated map")
	}
	return tm, nil
}

// DefaultDBConnectionResolver implements adaptor.DBConnectionResolver
type DefaultDBConnectionResolver struct {
	providers map[string]adaptor.DBProvider
	cfg       *config.Config
}

// NewDefaultDBConnectionResolver creates a new DefaultDBConnectionResolver.
// 循環参照を解消するため、DBProvidersをリストとして受け取り、内部でマップに変換します。
func NewDefaultDBConnectionResolver(p struct {
	fx.In
	DBProviders []adaptor.DBProvider `group:"db_providers"` // リストとして受け取る
	Cfg         *config.Config
}) adaptor.DBConnectionResolver {
	providerMap := make(map[string]adaptor.DBProvider)
	for _, provider := range p.DBProviders {
		providerMap[provider.Type()] = provider
	}
	return &DefaultDBConnectionResolver{
		providers: providerMap,
		cfg:       p.Cfg,
	}
}

// ResolveDBConnection resolves the database connection instance by name.
func (r *DefaultDBConnectionResolver) ResolveDBConnection(ctx context.Context, name string) (adaptor.DBConnection, error) {
	dbConfig, ok := r.cfg.Surfin.Datasources[name]
	if !ok {
		return nil, fmt.Errorf("database configuration '%s' not found", name)
	}

	provider, ok := r.providers[dbConfig.Type]
	if !ok {
		// Redshift のような特殊なケースを考慮
		if dbConfig.Type == "redshift" {
			provider, ok = r.providers["postgres"]
		}
		if !ok {
			return nil, fmt.Errorf("DBProvider for type '%s' not found", dbConfig.Type)
		}
	}

	// Provider を使用して接続を取得
	// Provider は内部で接続プールを管理し、必要に応じて RefreshConnection を呼び出す
	conn, err := provider.GetConnection(name)
	if err != nil {
		return nil, fmt.Errorf("failed to get connection '%s': %w", name, err)
	}
	
	// 接続が有効であることを確認するために RefreshConnection を呼び出す (オプションだが、ここでは省略)
	// conn.RefreshConnection(ctx) // GetConnection が最新の有効な接続を返すことを期待

	return conn, nil
}

// Module defines the application's Fx module.
// Add application-specific dependency injection settings here.
var Module = fx.Options(
	// DB Provider Modules
	gormadaptor.Module, // NewGormTransactionManagerFactory の提供のみ

	// Provide the aggregated map[string]adaptor.DBConnection and map[string]tx.TransactionManager
	// NewDBConnectionsAndTxManagers は map[string]adaptor.DBProvider も提供する
	fx.Provide(NewDBConnectionsAndTxManagers), // 修正
	
	// Provide the specific metadata TxManager required by JobFactory
	fx.Provide(fx.Annotate(
		NewMetadataTxManager,
		fx.ResultTags(`name:"metadata"`), // JobFactoryParamsが要求する名前付きインスタンス
	)),
	
	// Provide the concrete DBConnectionResolver implementation
	fx.Provide(fx.Annotate(
		NewDefaultDBConnectionResolver,
		fx.As(new(adaptor.DBConnectionResolver)),
	)),

	// Provide application migration FS by name
	fx.Provide(
		fx.Annotate(
			func(params struct {
				fx.In
				RawAppMigrationsFS embed.FS `name:"rawApplicationMigrationsFS"` // Raw embed.FS injected from main.go
			}) fs.FS {
				// Due to 'go:embed all:resources/migrations', the 'resources' directory is created at the root of the FS.
				// Remove this prefix so the framework can directly reference 'postgres' or 'mysql'.
				subFS, err := fs.Sub(params.RawAppMigrationsFS, "resources/migrations")
				if err != nil {
					// The go:embed path is fixed, so this should not normally be reached, but panic just in case.
					logger.Fatalf("Failed to create subdirectory for application migration FS: %v", err)
				}
				// Return fs.FS.
				return subFS
			},
			// Tag the result with the name 'weatherAppFS'.
			fx.ResultTags(`name:"weatherAppFS"`),
		),
	),
	
	// Provide the aggregated map[string]fs.FS
	fx.Provide(fx.Annotate(
		NewMigrationFSMap,
		fx.ResultTags(`name:"allMigrationFS"`),
	)),
	migrationfs.Module, // フレームワークマイグレーションFSの提供をアプリケーション側で明示的に行う
)
