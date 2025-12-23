package config

import (
	"fmt"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

// Config holds application configuration
type Config struct {
	Database DatabaseConfig
	Worker   WorkerConfig
	Output   OutputConfig
}

// DatabaseConfig holds database connection settings
type DatabaseConfig struct {
	Type     string
	Host     string
	Port     int
	User     string
	Password string
	Database string
}

// WorkerConfig holds worker pool settings
type WorkerConfig struct {
	NumWorkers int
	BatchSize  int64
}

// OutputConfig holds output settings
type OutputConfig struct {
	Format string
}

// Load reads configuration from environment variables
func Load() (*Config, error) {
	// Try to load .env file (optional)
	_ = godotenv.Load()

	cfg := &Config{
		Database: DatabaseConfig{
			Type:     getEnv("DB_TYPE", "postgres"),
			Host:     getEnv("POSTGRES_HOST", "localhost"),
			Port:     getEnvInt("POSTGRES_PORT", 5432),
			User:     getEnv("POSTGRES_USER", "postgres"),
			Password: getEnv("POSTGRES_PASSWORD", "postgres"),
			Database: getEnv("POSTGRES_DB", "django_app"),
		},
		Worker: WorkerConfig{
			NumWorkers: getEnvInt("WORKERS", 8),
			BatchSize:  int64(getEnvInt("BATCH_SIZE", 1000)),
		},
		Output: OutputConfig{
			Format: getEnv("OUTPUT_FORMAT", "json"),
		},
	}

	return cfg, cfg.Validate()
}

// Validate checks if configuration is valid
func (c *Config) Validate() error {
	if c.Database.Type != "postgres" && c.Database.Type != "mysql" {
		return fmt.Errorf("invalid database type: %s", c.Database.Type)
	}

	if c.Worker.NumWorkers < 1 || c.Worker.NumWorkers > 128 {
		return fmt.Errorf("invalid number of workers: %d (must be 1-128)", c.Worker.NumWorkers)
	}

	if c.Worker.BatchSize < 100 || c.Worker.BatchSize > 10000 {
		return fmt.Errorf("invalid batch size: %d (must be 100-10000)", c.Worker.BatchSize)
	}

	if c.Output.Format != "json" && c.Output.Format != "csv" {
		return fmt.Errorf("invalid output format: %s", c.Output.Format)
	}

	return nil
}

// getEnv reads an environment variable with a default value
func getEnv(key, defaultValue string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return defaultValue
}

// getEnvInt reads an integer environment variable with a default value
func getEnvInt(key string, defaultValue int) int {
	if value := os.Getenv(key); value != "" {
		if intValue, err := strconv.Atoi(value); err == nil {
			return intValue
		}
	}
	return defaultValue
}
