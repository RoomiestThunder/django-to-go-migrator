package database

import (
	"database/sql"
	"fmt"
	"sync"

	_ "github.com/go-sql-driver/mysql"
	_ "github.com/lib/pq"
)

// ConnectionPool manages database connections
type ConnectionPool struct {
	DB    *sql.DB
	mutex sync.RWMutex
}

// PostgresConfig contains PostgreSQL connection configuration
type PostgresConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

// MySQLConfig contains MySQL connection configuration
type MySQLConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

// NewPostgresConnection creates a connection to PostgreSQL
func NewPostgresConnection(cfg PostgresConfig) (*ConnectionPool, error) {
	dsn := fmt.Sprintf("host=%s port=%d user=%s password=%s dbname=%s sslmode=disable",
		cfg.Host, cfg.Port, cfg.User, cfg.Password, cfg.Database)

	db, err := sql.Open("postgres", dsn)
	if err != nil {
		return nil, fmt.Errorf("error connecting to PostgreSQL: %w", err)
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("error verifying connection: %w", err)
	}

	// Optimization for handling large volumes
	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(10)

	return &ConnectionPool{DB: db}, nil
}

// NewMySQLConnection creates a connection to MySQL
func NewMySQLConnection(cfg MySQLConfig) (*ConnectionPool, error) {
	dsn := fmt.Sprintf("%s:%s@tcp(%s:%d)/%s",
		cfg.User, cfg.Password, cfg.Host, cfg.Port, cfg.Database)

	db, err := sql.Open("mysql", dsn)
	if err != nil {
		return nil, fmt.Errorf("error connecting to MySQL: %w", err)
	}

	// Verify connection
	if err := db.Ping(); err != nil {
		return nil, fmt.Errorf("error verifying connection: %w", err)
	}

	db.SetMaxOpenConns(100)
	db.SetMaxIdleConns(10)

	return &ConnectionPool{DB: db}, nil
}

// Close closes the database connection
func (cp *ConnectionPool) Close() error {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	if cp.DB != nil {
		return cp.DB.Close()
	}
	return nil
}

// GetTableCount returns the number of rows in a table
func (cp *ConnectionPool) GetTableCount(tableName string) (int64, error) {
	var count int64
	query := fmt.Sprintf("SELECT COUNT(*) FROM %s", tableName)
	err := cp.DB.QueryRow(query).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("error counting rows in %s: %w", tableName, err)
	}
	return count, nil
}

// GetTableNames returns a list of tables in the database
func (cp *ConnectionPool) GetTableNames(dbType string) ([]string, error) {
	var query string

	switch dbType {
	case "postgres":
		query = `
			SELECT table_name FROM information_schema.tables 
			WHERE table_schema = 'public' 
			AND table_type = 'BASE TABLE'
			AND table_name NOT LIKE 'django_%'
			AND table_name NOT LIKE 'auth_%'
		`
	case "mysql":
		query = `
			SELECT TABLE_NAME FROM information_schema.TABLES 
			WHERE TABLE_SCHEMA = DATABASE()
			AND TABLE_NAME NOT LIKE 'django_%'
			AND TABLE_NAME NOT LIKE 'auth_%'
		`
	default:
		return nil, fmt.Errorf("unknown database type: %s", dbType)
	}

	rows, err := cp.DB.Query(query)
	if err != nil {
		return nil, fmt.Errorf("error retrieving tables: %w", err)
	}
	defer rows.Close()

	var tables []string
	for rows.Next() {
		var tableName string
		if err := rows.Scan(&tableName); err != nil {
			return nil, err
		}
		tables = append(tables, tableName)
	}

	return tables, rows.Err()
}
