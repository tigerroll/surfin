# Database Adapter Auto-Scan Feature Design

This document describes the design for adding an automatic scanning feature from `*sql.Rows` to Go structs within the database adapter layer of the Surfin batch framework. This aims to reduce the manual writing of `scanFunc` in the application layer and enable more generic data mapping.

## 1. Purpose

*   To simplify the writing of `scanFunc` for mapping `sql.Rows` to Go struct `T` in `GenericParquetExportTasklet` and other database readers.
*   To reduce boilerplate code that manually writes `rows.Scan()` for each entity type in the application layer.
*   To enhance data retrieval abstraction by providing an automatic scanning feature to Go structs within the database adapter layer.

## 2. Design Principles

*   **Extension of `DBConnection` Interface**: Add a generic scanning feature to the database connection interface.
*   **Leveraging ORM Capabilities**: Utilize the automatic mapping features provided by each ORM (e.g., GORM, sqlx) within their respective database adapter implementations.
*   **Clear Separation of Concerns**: Abstract data retrieval from the database and mapping to Go structs as a responsibility of the database adapter layer.
*   **Maintain Compatibility**: Ensure overall framework compatibility by having non-ORM dependent adapters (e.g., `dummy`) implement the interface and return appropriate errors if the feature is not supported.

## 3. Interface Changes

`ScanRowsToStruct` メソッドは、`pkg/batch/adapter/database/interfaces.go` の `DBConnection` インターフェースにすでに追加されています。

```go
// pkg/batch/adapter/database/interfaces.go

type DBConnection interface {
	// ... existing methods ...

	// ScanRowsToStruct scans the current row of *sql.Rows into the specified Go struct (dest).
	// This method leverages the automatic mapping capabilities of the ORM or library used internally by the adapter.
	// 'dest' must be a pointer to a struct.
	ScanRowsToStruct(rows *sql.Rows, dest interface{}) error
}
```

## 4. Adapter Implementation

`DBConnection` インターフェースを実装する各アダプターは、`ScanRowsToStruct` メソッドをすでに実装しています。

### 4.1. GORM-based Adapter (e.g., `pkg/batch/adapter/database/gorm`)

GORM provides a `ScanRows(rows *sql.Rows, dest interface{}) error` method on the `*gorm.DB` object, which can be used for implementation.

```go
// pkg/batch/adapter/database/gorm/connection.go (example)

func (c *gormDBConnection) ScanRowsToStruct(rows *sql.Rows, dest interface{}) error {
	// Use GORM's ScanRows method to scan *sql.Rows into dest.
	// c.db is the *gorm.DB instance.
	return c.db.ScanRows(rows, dest)
}
```

*   **Go Struct Preparation**: To utilize GORM's automatic mapping, Go structs must have appropriate GORM tags (e.g., `gorm:"column:time"`) configured.

### 4.2. Non-ORM Adapter (e.g., `pkg/batch/adapter/database/dummy`)

Adapters that do not provide this functionality should implement the method and return an error indicating that it is not supported.

```go
// pkg/batch/adapter/database/dummy/connection.go (example)

func (d *dummyDBConnection) ScanRowsToStruct(rows *sql.Rows, dest interface{}) error {
	// This feature is not supported by the dummy connection.
	return fmt.Errorf("ScanRowsToStruct is not supported by the dummy DB connection")
}
```

## 5. Usage in `GenericParquetExportTasklet` and `scanFunc`

`GenericParquetExportTasklet` の `Execute` メソッド内では、データベース接続を解決した後、`DBConnection` インスタンスの `ScanRowsToStruct` メソッドを使用して `scanFunc` が動的に生成されるようにすでに実装されています。

```go
// pkg/batch/component/tasklet/generic/parquet_export_tasklet.go (inside Execute method)

func (t *GenericParquetExportTasklet[T]) Execute(ctx context.Context, stepExecution *model.StepExecution) (model.ExitStatus, error) {
	// ... (existing database connection resolution logic) ...
	dbConn, err := t.dbConnectionResolver.ResolveDBConnection(ctx, t.config.DbRef)
	if err != nil {
		// ... error handling ...
	}
	defer func() {
		if closeErr := dbConn.Close(); closeErr != nil {
			logger.Errorf("Failed to close database connection '%s': %v", t.config.DbRef, closeErr)
		}
	}()

	sqlDB, err := dbConn.GetSQLDB()
	if err != nil {
		// ... error handling ...
	}

	// SQL query construction ...

	// Dynamically generate scanFunc using dbConn.ScanRowsToStruct.
	// The scanFunc field in GenericParquetExportTasklet will no longer be needed.
	currentScanFunc := func(rows *sql.Rows) (T, error) {
		var item T
		// Use the dbConn resolved within the Execute method's scope.
		if err := dbConn.ScanRowsToStruct(rows, &item); err != nil {
			return item, err
		}
		return item, nil
	}

	// Initialize SqlCursorReader.
	sqlReader := reader.NewSqlCursorReader[T](
		sqlDB,
		fmt.Sprintf("%s_sql_reader", stepExecution.StepName),
		sqlQuery,
		nil,
		currentScanFunc, // Pass the dynamically generated scanFunc.
	)

	// ... (subsequent processing) ...
}
```

*   The `scanFunc func(rows *sql.Rows) (T, error)` field will be removed from the `GenericParquetExportTasklet` struct.
*   The `scanFunc` argument will be removed from the `NewGenericParquetExportTasklet` function.
*   It will no longer be necessary to pass `scanFunc` when calling `NewGenericParquetExportTaskletBuilder` (e.g., in `example/weather/internal/component/tasklet/module.go`).

## 6. Considerations and Trade-offs

*   **ORM-Specific Tags**: Go structs will require ORM-specific tags (e.g., GORM's `gorm:"column:..."`, `sqlx`'s `db:"..."`) depending on the ORM used. This preparation will be done at the application layer.
*   **Custom Type Conversion**: Custom type conversions not natively supported by the ORM (e.g., `time.Time` to `int64` Unix milliseconds) will need to be configured using ORM-provided extension features (custom scanners, value converters, etc.). This may deepen the dependency on the ORM.
*   **Error Handling**: If an adapter that does not support `ScanRowsToStruct` (e.g., `dummy`) is used, an error will occur at runtime. The caller (inside `scanFunc`) must handle this error appropriately.
*   **Interface Bloat**: The potential for interface bloat by adding ORM-specific functionality to the `DBConnection` interface is a trade-off. However, since this feature abstracts a core part of data retrieval from the database, it is considered acceptable.
*   **`SqlCursorReader`'s `db` Field**: `SqlCursorReader` accepts `*sql.DB`. Since `ScanRowsToStruct` is a method of `DBConnection`, `dbConn` will be resolved within the `Execute` method, and the `scanFunc` that captures this `dbConn` in a closure will be passed to `SqlCursorReader`.
