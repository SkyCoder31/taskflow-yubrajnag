package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

type Config struct {
	Server   ServerConfig
	Database DatabaseConfig
	Auth     AuthConfig
}

type ServerConfig struct {
	Port         int
	ReadTimeout  time.Duration
	WriteTimeout time.Duration
}

type DatabaseConfig struct {
	Host     string
	Port     int
	User     string
	Password string
	Name     string
	SSLMode  string
}

type AuthConfig struct {
	JWTSecret  string
	JWTExpiry  time.Duration
	BcryptCost int
}

func (d DatabaseConfig) DSN() string {
	return fmt.Sprintf(
		"postgres://%s:%s@%s:%d/%s?sslmode=%s",
		d.User, d.Password, d.Host, d.Port, d.Name, d.SSLMode,
	)
}

type Option func(*Config)

func WithBcryptCost(cost int) Option {
	return func(c *Config) { c.Auth.BcryptCost = cost }
}

func WithJWTExpiry(d time.Duration) Option {
	return func(c *Config) { c.Auth.JWTExpiry = d }
}

func WithServerPort(port int) Option {
	return func(c *Config) { c.Server.Port = port }
}

func Load(opts ...Option) (*Config, error) {
	cfg := &Config{
		Server: ServerConfig{
			Port:         envInt("SERVER_PORT", 8080),
			ReadTimeout:  envDuration("SERVER_READ_TIMEOUT", 5*time.Second),
			WriteTimeout: envDuration("SERVER_WRITE_TIMEOUT", 10*time.Second),
		},
		Database: DatabaseConfig{
			Host:     envStr("DB_HOST", "localhost"),
			Port:     envInt("DB_PORT", 5432),
			User:     envStr("DB_USER", "taskflow"),
			Password: envStr("DB_PASSWORD", "taskflow"),
			Name:     envStr("DB_NAME", "taskflow"),
			SSLMode:  envStr("DB_SSLMODE", "disable"),
		},
		Auth: AuthConfig{
			JWTSecret:  envStr("JWT_SECRET", ""),
			JWTExpiry:  envDuration("JWT_EXPIRY", 24*time.Hour),
			BcryptCost: envInt("BCRYPT_COST", 12),
		},
	}

	for _, opt := range opts {
		opt(cfg)
	}

	if cfg.Auth.JWTSecret == "" {
		return nil, fmt.Errorf("JWT_SECRET environment variable is required")
	}

	return cfg, nil
}

func envStr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func envInt(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return fallback
	}
	return n
}

func envDuration(key string, fallback time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	d, err := time.ParseDuration(v)
	if err != nil {
		return fallback
	}
	return d
}
