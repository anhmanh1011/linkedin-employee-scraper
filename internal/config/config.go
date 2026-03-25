package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

type Config struct {
	DfsLogin      string
	DfsPassword   string
	PostbackURL   string
	Depth         int
	BatchSize     int
	BatchDelayMs  int
	MaxConcurrent int
	RetryCount    int
	InputFile     string
	OutputFile    string
	StateFile     string
	RunDir        string
	ReceiverPort  string
}

func Load() (*Config, error) {
	_ = godotenv.Load()

	dataDir := getEnvDefault("DATA_DIR", "data")
	runDir := filepath.Join(dataDir, "runs", time.Now().Format("2006-01-02_15-04-05"))
	if err := os.MkdirAll(runDir, 0755); err != nil {
		return nil, fmt.Errorf("failed to create run directory %s: %w", runDir, err)
	}

	cfg := &Config{
		DfsLogin:     os.Getenv("DFS_LOGIN"),
		DfsPassword:  os.Getenv("DFS_PASSWORD"),
		PostbackURL:  os.Getenv("POSTBACK_URL"),
		InputFile:    getEnvDefault("INPUT_FILE", filepath.Join(dataDir, "input.txt")),
		OutputFile:   getEnvDefault("OUTPUT_FILE", filepath.Join(runDir, "output.txt")),
		StateFile:    getEnvDefault("STATE_FILE", filepath.Join(dataDir, "state.json")),
		RunDir:       runDir,
		ReceiverPort: getEnvDefault("RECEIVER_PORT", "8080"),
	}

	var err error
	cfg.Depth, err = getEnvInt("DEPTH", 700)
	if err != nil {
		return nil, fmt.Errorf("invalid DEPTH: %w", err)
	}
	cfg.BatchSize, err = getEnvInt("BATCH_SIZE", 100)
	if err != nil {
		return nil, fmt.Errorf("invalid BATCH_SIZE: %w", err)
	}
	cfg.BatchDelayMs, err = getEnvInt("BATCH_DELAY_MS", 500)
	if err != nil {
		return nil, fmt.Errorf("invalid BATCH_DELAY_MS: %w", err)
	}
	cfg.MaxConcurrent, err = getEnvInt("MAX_CONCURRENT", 30)
	if err != nil {
		return nil, fmt.Errorf("invalid MAX_CONCURRENT: %w", err)
	}
	cfg.RetryCount, err = getEnvInt("RETRY_COUNT", 3)
	if err != nil {
		return nil, fmt.Errorf("invalid RETRY_COUNT: %w", err)
	}

	if cfg.DfsLogin == "" || cfg.DfsPassword == "" {
		return nil, fmt.Errorf("DFS_LOGIN and DFS_PASSWORD are required")
	}
	if cfg.PostbackURL == "" {
		return nil, fmt.Errorf("POSTBACK_URL is required")
	}

	return cfg, nil
}

func getEnvDefault(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}

func getEnvInt(key string, defaultVal int) (int, error) {
	v := os.Getenv(key)
	if v == "" {
		return defaultVal, nil
	}
	return strconv.Atoi(v)
}
