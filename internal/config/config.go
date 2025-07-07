package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/spf13/viper"
)

type Config struct {
	Port         int    `mapstructure:"port"`
	DatabasePath string `mapstructure:"database_path"`
	LogLevel     string `mapstructure:"log_level"`
	WorkerThreads int   `mapstructure:"worker_threads"`
	BatchSize    int    `mapstructure:"batch_size"`
	SnapshotThreshold float64 `mapstructure:"snapshot_threshold"`
	RelayEndpoint string `mapstructure:"relay_endpoint"`
}

func Load(configPath string) (*Config, error) {
	// Set defaults
	viper.SetDefault("port", 9000)
	viper.SetDefault("database_path", "./stag-data")
	viper.SetDefault("log_level", "info")
	viper.SetDefault("worker_threads", 4)
	viper.SetDefault("batch_size", 50)
	viper.SetDefault("snapshot_threshold", 0.1)
	viper.SetDefault("relay_endpoint", "http://localhost:9000/api/v1/ingest")

	// Environment variables
	viper.SetEnvPrefix("STAG")
	viper.SetEnvKeyReplacer(strings.NewReplacer(".", "_"))
	viper.AutomaticEnv()

	// Configuration file
	if configPath != "" {
		viper.SetConfigFile(configPath)
	} else {
		// Search for config file in multiple locations
		viper.SetConfigName("config")
		viper.SetConfigType("yaml")
		viper.AddConfigPath(".")
		viper.AddConfigPath("./config")
		viper.AddConfigPath("$HOME/.stag")
		viper.AddConfigPath("/etc/stag")
	}

	// Read config file
	if err := viper.ReadInConfig(); err != nil {
		if _, ok := err.(viper.ConfigFileNotFoundError); !ok {
			return nil, fmt.Errorf("failed to read config file: %w", err)
		}
	}

	// Override with environment variables
	if port := os.Getenv("STAG_PORT"); port != "" {
		if p, err := strconv.Atoi(port); err == nil {
			viper.Set("port", p)
		}
	}

	if dbPath := os.Getenv("STAG_DATABASE_PATH"); dbPath != "" {
		viper.Set("database_path", dbPath)
	}

	if logLevel := os.Getenv("STAG_LOG_LEVEL"); logLevel != "" {
		viper.Set("log_level", logLevel)
	}

	if workers := os.Getenv("STAG_WORKER_THREADS"); workers != "" {
		if w, err := strconv.Atoi(workers); err == nil {
			viper.Set("worker_threads", w)
		}
	}

	if batchSize := os.Getenv("STAG_BATCH_SIZE"); batchSize != "" {
		if b, err := strconv.Atoi(batchSize); err == nil {
			viper.Set("batch_size", b)
		}
	}

	if threshold := os.Getenv("STAG_SNAPSHOT_THRESHOLD"); threshold != "" {
		if t, err := strconv.ParseFloat(threshold, 64); err == nil {
			viper.Set("snapshot_threshold", t)
		}
	}

	if endpoint := os.Getenv("STAG_RELAY_ENDPOINT"); endpoint != "" {
		viper.Set("relay_endpoint", endpoint)
	}

	// Unmarshal configuration
	var cfg Config
	if err := viper.Unmarshal(&cfg); err != nil {
		return nil, fmt.Errorf("failed to unmarshal config: %w", err)
	}

	// Validate configuration
	if err := cfg.Validate(); err != nil {
		return nil, fmt.Errorf("invalid configuration: %w", err)
	}

	// Ensure database directory exists
	if err := os.MkdirAll(filepath.Dir(cfg.DatabasePath), 0755); err != nil {
		return nil, fmt.Errorf("failed to create database directory: %w", err)
	}

	return &cfg, nil
}

func (c *Config) Validate() error {
	if c.Port < 1 || c.Port > 65535 {
		return fmt.Errorf("port must be between 1 and 65535, got %d", c.Port)
	}

	if c.DatabasePath == "" {
		return fmt.Errorf("database_path cannot be empty")
	}

	validLogLevels := []string{"debug", "info", "warn", "error"}
	validLogLevel := false
	for _, level := range validLogLevels {
		if c.LogLevel == level {
			validLogLevel = true
			break
		}
	}
	if !validLogLevel {
		return fmt.Errorf("log_level must be one of %v, got %s", validLogLevels, c.LogLevel)
	}

	if c.WorkerThreads < 1 || c.WorkerThreads > 100 {
		return fmt.Errorf("worker_threads must be between 1 and 100, got %d", c.WorkerThreads)
	}

	if c.BatchSize < 1 || c.BatchSize > 1000 {
		return fmt.Errorf("batch_size must be between 1 and 1000, got %d", c.BatchSize)
	}

	if c.SnapshotThreshold < 0 || c.SnapshotThreshold > 1 {
		return fmt.Errorf("snapshot_threshold must be between 0 and 1, got %f", c.SnapshotThreshold)
	}

	return nil
}

func (c *Config) String() string {
	return fmt.Sprintf("Config{Port: %d, DatabasePath: %s, LogLevel: %s, WorkerThreads: %d, BatchSize: %d, SnapshotThreshold: %.2f, RelayEndpoint: %s}",
		c.Port, c.DatabasePath, c.LogLevel, c.WorkerThreads, c.BatchSize, c.SnapshotThreshold, c.RelayEndpoint)
}