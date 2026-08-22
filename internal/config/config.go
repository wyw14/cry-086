package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	Environment string
	HTTP        HTTP
	Storage     Storage
	Credentials Credentials
	SeedDemo    bool
}

type HTTP struct {
	Address         string
	RequestTimeout  time.Duration
	ShutdownTimeout time.Duration
	AllowedOrigins  []string
	WebAssets       string
}

type Storage struct {
	Mode             string
	PostgresURL      string
	EvidenceRoot     string
	MaxEvidenceBytes int64
}

type Credentials struct {
	AccessTokenSecret string
	SimulatorKey      string
}

type environment struct {
	lookup func(string) (string, bool)
}

func Load() (Config, error) {
	return readEnvironment(environment{lookup: os.LookupEnv})
}

func readEnvironment(source environment) (Config, error) {
	requestTimeout, err := source.positiveDuration("REQUEST_TIMEOUT", 5*time.Second)
	if err != nil {
		return Config{}, err
	}
	shutdownTimeout, err := source.positiveDuration("SHUTDOWN_TIMEOUT", 15*time.Second)
	if err != nil {
		return Config{}, err
	}
	maxEvidenceBytes, err := source.positiveInt("FILE_MAX_BYTES", 10*1024*1024)
	if err != nil {
		return Config{}, err
	}
	seedDemo, err := source.boolean("SEED_DEMO", true)
	if err != nil {
		return Config{}, err
	}

	result := Config{
		Environment: source.text("APP_ENV", "development"),
		HTTP: HTTP{
			Address:         source.text("LISTEN_ADDRESS", ":8080"),
			RequestTimeout:  requestTimeout,
			ShutdownTimeout: shutdownTimeout,
			AllowedOrigins:  source.list("ALLOWED_ORIGINS", "http://localhost:5173"),
			WebAssets:       source.text("WEB_DIST", "./web/dist"),
		},
		Storage: Storage{
			Mode:             source.text("REPOSITORY_MODE", "memory"),
			PostgresURL:      source.text("DATABASE_URL", ""),
			EvidenceRoot:     source.text("FILE_ROOT", "./var/files"),
			MaxEvidenceBytes: maxEvidenceBytes,
		},
		Credentials: Credentials{
			AccessTokenSecret: source.text("ACCESS_TOKEN_SECRET", "local-development-secret-change-me"),
			SimulatorKey:      source.text("SIMULATOR_KEY", "local-simulator-key"),
		},
		SeedDemo: seedDemo,
	}
	if err := result.validate(); err != nil {
		return Config{}, err
	}
	return result, nil
}

func (c Config) validate() error {
	switch c.Storage.Mode {
	case "memory":
	case "postgres":
		if c.Storage.PostgresURL == "" {
			return fmt.Errorf("DATABASE_URL is required in postgres mode")
		}
	default:
		return fmt.Errorf("REPOSITORY_MODE must be memory or postgres")
	}
	if len(c.Credentials.AccessTokenSecret) < 24 {
		return fmt.Errorf("ACCESS_TOKEN_SECRET must contain at least 24 characters")
	}
	if len(c.HTTP.AllowedOrigins) == 0 {
		return fmt.Errorf("ALLOWED_ORIGINS must contain at least one origin")
	}
	return nil
}

func (e environment) raw(key string) string {
	value, ok := e.lookup(key)
	if !ok {
		return ""
	}
	return strings.TrimSpace(value)
}

func (e environment) text(key, fallback string) string {
	if value := e.raw(key); value != "" {
		return value
	}
	return fallback
}

func (e environment) positiveDuration(key string, fallback time.Duration) (time.Duration, error) {
	raw := e.raw(key)
	if raw == "" {
		return fallback, nil
	}
	value, err := time.ParseDuration(raw)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration", key)
	}
	return value, nil
}

func (e environment) positiveInt(key string, fallback int64) (int64, error) {
	raw := e.raw(key)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer", key)
	}
	return value, nil
}

func (e environment) boolean(key string, fallback bool) (bool, error) {
	raw := e.raw(key)
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return false, fmt.Errorf("%s must be true or false", key)
	}
	return value, nil
}

func (e environment) list(key, fallback string) []string {
	raw := e.text(key, fallback)
	values := make([]string, 0, strings.Count(raw, ",")+1)
	for _, candidate := range strings.Split(raw, ",") {
		if candidate = strings.TrimSpace(candidate); candidate != "" {
			values = append(values, candidate)
		}
	}
	return values
}
