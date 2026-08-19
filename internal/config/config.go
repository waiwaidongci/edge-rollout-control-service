package config

import (
	"errors"
	"fmt"
	"os"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

type Config struct {
	Server   ServerConfig   `yaml:"server"`
	Database DatabaseConfig `yaml:"database"`
	Worker   WorkerConfig   `yaml:"worker"`
	Logging  LoggingConfig  `yaml:"logging"`
}

type ServerConfig struct {
	Address      string        `yaml:"address"`
	ReadTimeout  time.Duration `yaml:"read_timeout"`
	WriteTimeout time.Duration `yaml:"write_timeout"`
	IdleTimeout  time.Duration `yaml:"idle_timeout"`
	BodyLimit    int64         `yaml:"body_limit"`
}

type DatabaseConfig struct {
	Driver string `yaml:"driver"`
	DSN    string `yaml:"dsn"`
}

type WorkerConfig struct {
	SchedulerInterval  time.Duration `yaml:"scheduler_interval"`
	WebhookInterval    time.Duration `yaml:"webhook_interval"`
	ReceiptTimeout     time.Duration `yaml:"receipt_timeout"`
	WebhookMaxAttempts int           `yaml:"webhook_max_attempts"`
}

type LoggingConfig struct {
	Level string `yaml:"level"`
}

func Default() Config {
	return Config{
		Server:   ServerConfig{Address: ":8080", ReadTimeout: 10 * time.Second, WriteTimeout: 20 * time.Second, IdleTimeout: 60 * time.Second, BodyLimit: 2 << 20},
		Database: DatabaseConfig{Driver: "sqlite", DSN: "data/edge-rollout.db"},
		Worker:   WorkerConfig{SchedulerInterval: time.Second, WebhookInterval: 2 * time.Second, ReceiptTimeout: 30 * time.Second, WebhookMaxAttempts: 8},
		Logging:  LoggingConfig{Level: "info"},
	}
}

func Load(path string) (Config, error) {
	cfg := Default()
	if path != "" {
		data, err := os.ReadFile(path)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return Config{}, fmt.Errorf("read config: %w", err)
		}
		if len(data) > 0 {
			if err := yaml.Unmarshal(data, &cfg); err != nil {
				return Config{}, fmt.Errorf("decode config: %w", err)
			}
		}
	}
	applyEnvironment(&cfg)
	if err := cfg.Validate(); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func applyEnvironment(cfg *Config) {
	stringOverride("SERVER_ADDR", &cfg.Server.Address)
	stringOverride("DATABASE_DRIVER", &cfg.Database.Driver)
	stringOverride("DATABASE_DSN", &cfg.Database.DSN)
	stringOverride("LOG_LEVEL", &cfg.Logging.Level)
	durationOverride("SERVER_READ_TIMEOUT", &cfg.Server.ReadTimeout)
	durationOverride("SERVER_WRITE_TIMEOUT", &cfg.Server.WriteTimeout)
	durationOverride("WORKER_SCHEDULER_INTERVAL", &cfg.Worker.SchedulerInterval)
	durationOverride("WORKER_WEBHOOK_INTERVAL", &cfg.Worker.WebhookInterval)
	durationOverride("WORKER_RECEIPT_TIMEOUT", &cfg.Worker.ReceiptTimeout)
	if raw := os.Getenv("WEBHOOK_MAX_ATTEMPTS"); raw != "" {
		if parsed, err := strconv.Atoi(raw); err == nil {
			cfg.Worker.WebhookMaxAttempts = parsed
		}
	}
}

func stringOverride(key string, target *string) {
	if value := os.Getenv(key); value != "" {
		*target = value
	}
}

func durationOverride(key string, target *time.Duration) {
	if value := os.Getenv(key); value != "" {
		if parsed, err := time.ParseDuration(value); err == nil {
			*target = parsed
		}
	}
}

func (c Config) Validate() error {
	if c.Server.Address == "" {
		return errors.New("server address is required")
	}
	if c.Database.Driver != "sqlite" {
		return fmt.Errorf("unsupported database driver %q", c.Database.Driver)
	}
	if c.Database.DSN == "" {
		return errors.New("database dsn is required")
	}
	if c.Worker.SchedulerInterval <= 0 || c.Worker.WebhookInterval <= 0 {
		return errors.New("worker intervals must be positive")
	}
	return nil
}
