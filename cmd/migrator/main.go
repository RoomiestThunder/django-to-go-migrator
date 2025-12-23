package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"time"

	"django-to-go-migrator/internal/config"
	"django-to-go-migrator/internal/database"
	"django-to-go-migrator/internal/migrator"
	"django-to-go-migrator/internal/models"
)

const version = "1.0.0"

func main() {
	if err := run(); err != nil {
		log.Fatal(err)
	}
}

func run() error {
	// Parse command line flags
	cfg := parseFlags()

	fmt.Printf("Django to Go Migrator v%s\n\n", version)

	if cfg.demo {
		return runDemoMode(cfg.workers, cfg.batchSize)
	}

	return runMigration(cfg)
}

type cliConfig struct {
	source    string
	host      string
	port      int
	user      string
	password  string
	database  string
	workers   int
	batchSize int64
	output    string
	benchmark bool
	demo      bool
}

func parseFlags() *cliConfig {
	// Load environment configuration
	envCfg, _ := config.Load()

	cfg := &cliConfig{}
	flag.StringVar(&cfg.source, "source", "postgres", "Database type (postgres/mysql)")
	flag.StringVar(&cfg.host, "host", envCfg.Database.Host, "Database host")
	flag.IntVar(&cfg.port, "port", envCfg.Database.Port, "Database port")
	flag.StringVar(&cfg.user, "user", envCfg.Database.User, "Database user")
	flag.StringVar(&cfg.password, "password", envCfg.Database.Password, "Database password")
	flag.StringVar(&cfg.database, "database", envCfg.Database.Database, "Database name")
	flag.IntVar(&cfg.workers, "workers", envCfg.Worker.NumWorkers, "Number of workers")
	flag.Int64Var(&cfg.batchSize, "batch-size", envCfg.Worker.BatchSize, "Batch size")
	flag.StringVar(&cfg.output, "output", envCfg.Output.Format, "Output format (json/csv)")
	flag.BoolVar(&cfg.benchmark, "benchmark", false, "Run performance benchmark")
	flag.BoolVar(&cfg.demo, "demo", false, "Demo mode without database connection")

	flag.Parse()
	return cfg
}

func runMigration(cfg *cliConfig) error {
	fmt.Printf("Connecting to %s database...\n", cfg.source)

	// Initialize database connection
	pool, err := connectDatabase(cfg)
	if err != nil {
		return fmt.Errorf("failed to connect to database: %w", err)
	}
	defer pool.Close()

	fmt.Println("Connection established")

	// Get list of tables
	fmt.Println("\nRetrieving table list...")
	tables, err := pool.GetTableNames(cfg.source)
	if err != nil {
		return fmt.Errorf("failed to retrieve tables: %w", err)
	}

	if len(tables) == 0 {
		fmt.Println("No tables found")
		return nil
	}

	fmt.Printf("Tables found: %d\n", len(tables))
	for _, table := range tables {
		fmt.Printf("  - %s\n", table)
	}

	// Start migration
	fmt.Println("\nStarting migration process...")

	processor := migrator.NewDataProcessor(pool, cfg.source, cfg.workers, cfg.batchSize)
	ctx := context.Background()
	startTime := time.Now()

	for _, tableName := range tables {
		if err := processor.MigrateTable(ctx, tableName); err != nil {
			log.Printf("Error migrating %s: %v", tableName, err)
		}
	}

	// Get and print report
	report := processor.GetReport(cfg.source)
	duration := time.Since(startTime)

	printReport(report, duration)

	// Export results
	if cfg.output == "json" {
		fmt.Println("\nExporting to JSON...")
		if err := migrator.ExportJSON(report, "output/migration_report.json"); err != nil {
			log.Printf("Export error: %v", err)
		}
	}

	if cfg.benchmark {
		runBenchmark(cfg.workers, cfg.batchSize)
	}

	return nil
}

func connectDatabase(cfg *cliConfig) (*database.ConnectionPool, error) {
	switch cfg.source {
	case "postgres":
		return database.NewPostgresConnection(database.PostgresConfig{
			Host:     cfg.host,
			Port:     cfg.port,
			User:     cfg.user,
			Password: cfg.password,
			Database: cfg.database,
		})
	case "mysql":
		return database.NewMySQLConnection(database.MySQLConfig{
			Host:     cfg.host,
			Port:     cfg.port,
			User:     cfg.user,
			Password: cfg.password,
			Database: cfg.database,
		})
	default:
		return nil, fmt.Errorf("unknown database type: %s", cfg.source)
	}
}

func printReport(report *models.MigrationReport, duration time.Duration) {
	fmt.Println("\n" + repeatChar("=", 60))
	fmt.Println("MIGRATION REPORT")
	fmt.Println(repeatChar("=", 60))

	fmt.Println("\nMigration completed successfully")
	fmt.Println("\nStatistics:")
	fmt.Printf("  Execution time:        %v\n", duration)
	fmt.Printf("  Total rows:            %d\n", report.TotalRows)
	fmt.Printf("  Errors:                %d\n", report.TotalErrors)
	fmt.Printf("  Average (rows/sec):    %.0f\n", report.Performance.AverageRowsPerSecond)
	fmt.Printf("  Workers used:          %d\n\n", report.Performance.WorkersUsed)

	fmt.Println("By tables:")
	for _, table := range report.Tables {
		fmt.Printf("\n  %s:\n", table.TableName)
		fmt.Printf("    Total rows:      %d\n", table.TotalRows)
		fmt.Printf("    Migrated:        %d\n", table.MigratedRows)
		fmt.Printf("    Errors:          %d\n", table.ErrorCount)
		fmt.Printf("    Time:            %dms\n", table.DurationMS)
		fmt.Printf("    Speed:           %.0f rows/sec\n", table.RowsPerSecond)
	}

	fmt.Println("\n" + repeatChar("=", 60))
}

func runDemoMode(workers int, batchSize int64) error {
	fmt.Println("Demo mode (without database connection)")

	// Simulate processing 10000 rows
	processor := migrator.NewStreamProcessor(workers, batchSize, func(ctx context.Context, job interface{}) error {
		// Simulate processing
		time.Sleep(time.Microsecond * 100)
		return nil
	})

	fmt.Printf("Processing 10000 rows with %d workers...\n", workers)

	startTime := time.Now()
	for i := 0; i < 10000; i++ {
		processor.Submit(map[string]interface{}{
			"id":    i,
			"value": fmt.Sprintf("row_%d", i),
		})
	}

	processor.Wait()
	duration := time.Since(startTime)

	fmt.Printf("\nMigration completed in %v\n", duration)
	fmt.Printf("Performance: %.0f rows/sec\n", float64(10000)/duration.Seconds())

	return nil
}

func runBenchmark(workers int, batchSize int64) {
	fmt.Println("\nPerformance benchmark")

	sizes := []int{1000, 10000, 100000}

	fmt.Printf("%-10s | %-10s | %-15s\n", "Size", "Time (ms)", "Rows/sec")
	fmt.Println(repeatChar("-", 40))

	for _, size := range sizes {
		processor := migrator.NewStreamProcessor(workers, batchSize, func(ctx context.Context, job interface{}) error {
			time.Sleep(time.Microsecond * 50)
			return nil
		})

		startTime := time.Now()
		for i := 0; i < size; i++ {
			processor.Submit(i)
		}
		processor.Wait()
		duration := time.Since(startTime)

		rowsPerSec := float64(size) / duration.Seconds()
		fmt.Printf("%-10d | %-10d | %-15.0f\n", size, duration.Milliseconds(), rowsPerSec)
	}

	fmt.Println("\nBenchmark completed")
}

func repeatChar(s string, count int) string {
	result := ""
	for i := 0; i < count; i++ {
		result += s
	}
	return result
}
