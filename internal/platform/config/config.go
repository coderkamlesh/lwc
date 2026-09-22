package config

import (
	"bufio"
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	AppEnv           string
	HTTPAddr         string
	RequestTimeout   time.Duration
	TursoDatabaseURL string
	TursoAuthToken   string
	DBMaxOpenConns   int
	DBMaxIdleConns   int
	DBConnLifetime   time.Duration
}

func Load() (Config, error) {
	if isLocalEnvironment() {
		if err := loadLocalDotEnv(); err != nil {
			return Config{}, err
		}
	}

	cfg := Config{
		AppEnv:           envOrDefault("APP_ENV", "development"),
		HTTPAddr:         envOrDefault("HTTP_ADDR", ":8080"),
		TursoDatabaseURL: strings.TrimSpace(os.Getenv("TURSO_DATABASE_URL")),
		TursoAuthToken:   strings.TrimSpace(os.Getenv("TURSO_AUTH_TOKEN")),
	}

	if cfg.TursoDatabaseURL == "" {
		return Config{}, fmt.Errorf("TURSO_DATABASE_URL is required")
	}
	if cfg.TursoAuthToken == "" {
		return Config{}, fmt.Errorf("TURSO_AUTH_TOKEN is required")
	}

	var err error
	if cfg.RequestTimeout, err = durationEnv("HTTP_REQUEST_TIMEOUT", 10*time.Second); err != nil {
		return Config{}, err
	}
	if cfg.DBMaxOpenConns, err = intEnv("DB_MAX_OPEN_CONNS", 1); err != nil {
		return Config{}, err
	}
	if cfg.DBMaxIdleConns, err = intEnv("DB_MAX_IDLE_CONNS", 1); err != nil {
		return Config{}, err
	}
	if cfg.DBConnLifetime, err = durationEnv("DB_CONN_MAX_LIFETIME", 5*time.Minute); err != nil {
		return Config{}, err
	}

	if cfg.DBMaxOpenConns < 1 {
		return Config{}, fmt.Errorf("DB_MAX_OPEN_CONNS must be at least 1")
	}
	if cfg.DBMaxIdleConns < 0 || cfg.DBMaxIdleConns > cfg.DBMaxOpenConns {
		return Config{}, fmt.Errorf("DB_MAX_IDLE_CONNS must be between 0 and DB_MAX_OPEN_CONNS")
	}
	if cfg.RequestTimeout <= 0 {
		return Config{}, fmt.Errorf("HTTP_REQUEST_TIMEOUT must be greater than 0")
	}

	return cfg, nil
}

func isLocalEnvironment() bool {
	return os.Getenv("AWS_LAMBDA_RUNTIME_API") == "" && os.Getenv("AWS_LAMBDA_FUNCTION_NAME") == ""
}

// loadDotEnv loads local variables without overriding variables already set
// by the shell, CI, or the Lambda runtime.
func loadLocalDotEnv() error {
	workingDirectory, err := os.Getwd()
	if err != nil {
		return fmt.Errorf("find local .env: %w", err)
	}

	for directory := workingDirectory; ; directory = filepath.Dir(directory) {
		filename := filepath.Join(directory, ".env")
		if _, err := os.Stat(filename); err == nil {
			return loadDotEnv(filename)
		} else if !errors.Is(err, fs.ErrNotExist) {
			return fmt.Errorf("inspect %s: %w", filename, err)
		}

		parent := filepath.Dir(directory)
		if parent == directory {
			return nil
		}
	}
}

func loadDotEnv(filename string) error {
	file, err := os.Open(filename)
	if errors.Is(err, fs.ErrNotExist) {
		return nil
	}
	if err != nil {
		return fmt.Errorf("open %s: %w", filename, err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	lineNumber := 0
	for scanner.Scan() {
		lineNumber++
		line := strings.TrimSpace(scanner.Text())
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		line = strings.TrimPrefix(line, "export ")
		key, value, ok := strings.Cut(line, "=")
		key = strings.TrimSpace(key)
		if !ok || key == "" {
			return fmt.Errorf("invalid %s at line %d: expected KEY=VALUE", filename, lineNumber)
		}

		if _, exists := os.LookupEnv(key); exists {
			continue
		}

		value, err = parseDotEnvValue(strings.TrimSpace(value))
		if err != nil {
			return fmt.Errorf("invalid %s at line %d: %w", filename, lineNumber, err)
		}
		if err := os.Setenv(key, value); err != nil {
			return fmt.Errorf("set %s from %s: %w", key, filename, err)
		}
	}
	if err := scanner.Err(); err != nil {
		return fmt.Errorf("read %s: %w", filename, err)
	}

	return nil
}

func parseDotEnvValue(value string) (string, error) {
	if value == "" {
		return "", nil
	}

	switch value[0] {
	case '\'':
		if len(value) < 2 || value[len(value)-1] != '\'' {
			return "", fmt.Errorf("unterminated single-quoted value")
		}
		return value[1 : len(value)-1], nil
	case '"':
		if len(value) < 2 || value[len(value)-1] != '"' {
			return "", fmt.Errorf("unterminated double-quoted value")
		}
		parsed, err := strconv.Unquote(value)
		if err != nil {
			return "", fmt.Errorf("invalid double-quoted value: %w", err)
		}
		return parsed, nil
	default:
		return value, nil
	}
}

func envOrDefault(key, fallback string) string {
	if value := strings.TrimSpace(os.Getenv(key)); value != "" {
		return value
	}
	return fallback
}

func durationEnv(key string, fallback time.Duration) (time.Duration, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	duration, err := time.ParseDuration(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be a duration: %w", key, err)
	}
	return duration, nil
}

func intEnv(key string, fallback int) (int, error) {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(value)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer: %w", key, err)
	}
	return parsed, nil
}
