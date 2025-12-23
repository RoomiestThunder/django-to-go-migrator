package models

import (
	"time"
)

// User represents a Django user
type User struct {
	ID        int64     `json:"id"`
	Username  string    `json:"username"`
	Email     string    `json:"email"`
	FirstName string    `json:"first_name"`
	LastName  string    `json:"last_name"`
	IsActive  bool      `json:"is_active"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Post represents a blog post
type Post struct {
	ID        int64     `json:"id"`
	Title     string    `json:"title"`
	Content   string    `json:"content"`
	AuthorID  int64     `json:"author_id"`
	Published bool      `json:"published"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// Comment represents a post comment
type Comment struct {
	ID        int64     `json:"id"`
	Content   string    `json:"content"`
	PostID    int64     `json:"post_id"`
	AuthorID  int64     `json:"author_id"`
	CreatedAt time.Time `json:"created_at"`
	UpdatedAt time.Time `json:"updated_at"`
}

// MigrationJob represents a migration task for workers
type MigrationJob struct {
	ID     string
	Table  string
	Offset int64
	Limit  int64
	Data   []interface{}
	Error  error
}

// MigrationResult contains migration statistics for a table
type MigrationResult struct {
	TableName     string  `json:"table_name"`
	TotalRows     int64   `json:"total_rows"`
	MigratedRows  int64   `json:"migrated_rows"`
	ErrorCount    int64   `json:"error_count"`
	DurationMS    int64   `json:"duration_ms"`
	RowsPerSecond float64 `json:"rows_per_second"`
	ThroughputMB  float64 `json:"throughput_mb"`
}

// MigrationReport contains overall migration report
type MigrationReport struct {
	MigrationID string             `json:"migration_id"`
	StartTime   time.Time          `json:"start_time"`
	EndTime     time.Time          `json:"end_time"`
	Duration    time.Duration      `json:"duration_ms"`
	Source      string             `json:"source"`
	Tables      []MigrationResult  `json:"tables"`
	TotalRows   int64              `json:"total_rows"`
	TotalErrors int64              `json:"total_errors"`
	Performance PerformanceMetrics `json:"performance"`
}

// PerformanceMetrics contains performance statistics
type PerformanceMetrics struct {
	AverageRowsPerSecond float64 `json:"average_rows_per_second"`
	PeakMemoryMB         int64   `json:"peak_memory_mb"`
	CPUUsagePercent      float64 `json:"cpu_usage_percent"`
	WorkersUsed          int     `json:"workers_used"`
}
