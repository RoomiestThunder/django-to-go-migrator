package migrator

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"sync"
	"sync/atomic"
	"time"

	"django-to-go-migrator/internal/database"
	"django-to-go-migrator/internal/models"
)

// TableMigrator handles migration of a single table
type TableMigrator struct {
	name         string
	sourceDB     *database.ConnectionPool
	totalRows    int64
	migratedRows int64
	errorCount   int64
	startTime    time.Time
	endTime      time.Time
	data         []interface{}
	dataMutex    sync.RWMutex
}

// DataProcessor is the main migration processor
type DataProcessor struct {
	sourceDB          *database.ConnectionPool
	dbType            string
	numWorkers        int
	batchSize         int64
	results           map[string]*models.MigrationResult
	resultsMutex      sync.RWMutex
	startTime         time.Time
	errors            []error
	errorsMutex       sync.Mutex
	processedRowCount int64
}

// NewDataProcessor creates a new processor
func NewDataProcessor(
	sourceDB *database.ConnectionPool,
	dbType string,
	numWorkers int,
	batchSize int64,
) *DataProcessor {
	return &DataProcessor{
		sourceDB:   sourceDB,
		dbType:     dbType,
		numWorkers: numWorkers,
		batchSize:  batchSize,
		results:    make(map[string]*models.MigrationResult),
		startTime:  time.Now(),
	}
}

// MigrateTable migrates a single table
func (dp *DataProcessor) MigrateTable(ctx context.Context, tableName string) error {
	log.Printf("🔄 Starting migration of table: %s", tableName)

	tableMigrator := &TableMigrator{
		name:      tableName,
		sourceDB:  dp.sourceDB,
		startTime: time.Now(),
	}

	// Get row count in the table
	totalRows, err := dp.sourceDB.GetTableCount(tableName)
	if err != nil {
		return fmt.Errorf("error counting rows in %s: %w", tableName, err)
	}

	tableMigrator.totalRows = totalRows
	log.Printf("   📊 Total rows in %s: %d", tableName, totalRows)

	// Create worker pool
	processor := NewStreamProcessor(dp.numWorkers, dp.batchSize,
		dp.createProcessFunc(tableMigrator))

	// Read and process data in batches
	for offset := int64(0); offset < totalRows; offset += dp.batchSize {
		if err := ctx.Err(); err != nil {
			return err
		}

		limit := dp.batchSize
		if offset+limit > totalRows {
			limit = totalRows - offset
		}

		rows, err := dp.readTableBatch(tableName, offset, limit)
		if err != nil {
			dp.addError(fmt.Errorf("error reading batch from %s: %w", tableName, err))
			continue
		}

		for _, row := range rows {
			if err := processor.Submit(row); err != nil {
				dp.addError(err)
			}
		}
	}

	// Wait for processing to complete
	migrationErrors := processor.Wait()
	for _, err := range migrationErrors {
		dp.addError(err)
	}

	tableMigrator.endTime = time.Now()
	duration := tableMigrator.endTime.Sub(tableMigrator.startTime)

	// Save results
	result := &models.MigrationResult{
		TableName:     tableName,
		TotalRows:     tableMigrator.totalRows,
		MigratedRows:  tableMigrator.migratedRows,
		ErrorCount:    tableMigrator.errorCount,
		DurationMS:    duration.Milliseconds(),
		RowsPerSecond: float64(tableMigrator.migratedRows) / duration.Seconds(),
	}

	dp.resultsMutex.Lock()
	dp.results[tableName] = result
	dp.resultsMutex.Unlock()

	log.Printf("✅ Migration of %s completed in %v (%.0f rows/sec)",
		tableName, duration, result.RowsPerSecond)

	return nil
}

// readTableBatch reads a batch of data from a table
func (dp *DataProcessor) readTableBatch(tableName string, offset, limit int64) ([]interface{}, error) {
	query := fmt.Sprintf(
		"SELECT * FROM %s ORDER BY id LIMIT %d OFFSET %d",
		tableName, limit, offset,
	)

	rows, err := dp.sourceDB.DB.Query(query)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	columns, err := rows.Columns()
	if err != nil {
		return nil, err
	}

	var result []interface{}

	for rows.Next() {
		values := make([]interface{}, len(columns))
		valuePtrs := make([]interface{}, len(columns))

		for i := range columns {
			valuePtrs[i] = &values[i]
		}

		if err := rows.Scan(valuePtrs...); err != nil {
			return nil, err
		}

		rowMap := make(map[string]interface{})
		for i, col := range columns {
			rowMap[col] = values[i]
		}

		result = append(result, rowMap)
	}

	return result, rows.Err()
}

// createProcessFunc creates a processing function for the worker
func (dp *DataProcessor) createProcessFunc(tm *TableMigrator) ProcessFunc {
	return func(ctx context.Context, job interface{}) error {
		rowMap, ok := job.(map[string]interface{})
		if !ok {
			return fmt.Errorf("invalid data type")
		}

		// Transform and validate data
		if err := dp.validateRow(rowMap); err != nil {
			atomic.AddInt64(&tm.errorCount, 1)
			return nil // Continue despite error
		}

		// Save transformed data
		tm.dataMutex.Lock()
		tm.data = append(tm.data, rowMap)
		tm.dataMutex.Unlock()

		atomic.AddInt64(&tm.migratedRows, 1)
		atomic.AddInt64(&dp.processedRowCount, 1)

		return nil
	}
}

// validateRow validates a row of data
func (dp *DataProcessor) validateRow(row map[string]interface{}) error {
	// Basic validation - check required fields
	if id, ok := row["id"]; !ok || id == nil {
		return fmt.Errorf("missing ID")
	}

	return nil
}

// addError adds an error to the error list
func (dp *DataProcessor) addError(err error) {
	dp.errorsMutex.Lock()
	defer dp.errorsMutex.Unlock()
	dp.errors = append(dp.errors, err)
}

// GetReport returns a migration report
func (dp *DataProcessor) GetReport(source string) *models.MigrationReport {
	endTime := time.Now()
	duration := endTime.Sub(dp.startTime)

	dp.resultsMutex.RLock()
	tables := make([]models.MigrationResult, 0, len(dp.results))
	totalRows := int64(0)
	totalErrors := int64(0)

	for _, result := range dp.results {
		tables = append(tables, *result)
		totalRows += result.TotalRows
		totalErrors += result.ErrorCount
	}
	dp.resultsMutex.RUnlock()

	avgRowsPerSec := float64(totalRows) / duration.Seconds()

	return &models.MigrationReport{
		MigrationID: time.Now().Format("2006-01-02T15:04:05Z"),
		StartTime:   dp.startTime,
		EndTime:     endTime,
		Duration:    duration,
		Source:      source,
		Tables:      tables,
		TotalRows:   totalRows,
		TotalErrors: totalErrors,
		Performance: models.PerformanceMetrics{
			AverageRowsPerSecond: avgRowsPerSec,
			WorkersUsed:          dp.numWorkers,
		},
	}
}

// ExportJSON exports the report to JSON
func ExportJSON(report *models.MigrationReport, filepath string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return fmt.Errorf("error serializing JSON: %w", err)
	}

	// In a real application, this would write to a file
	log.Printf("📄 JSON report:\n%s", string(data))
	return nil
}
