package config

import (
	"os"
	"path/filepath"
	"time"
)

// Config holds runtime configuration sourced from environment variables.
type Config struct {
	Addr       string
	DataDir    string
	DBPath     string
	ReposDir   string
	LogsDir    string
	SessionTTL time.Duration
	MaxRetries int
}

func env(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// Load reads RepoHarbor configuration and creates the data directories.
func Load() (Config, error) {
	dataDir := env("REPOHARBOR_DATA_DIR", "./data")
	absData, err := filepath.Abs(dataDir)
	if err != nil {
		return Config{}, err
	}
	cfg := Config{
		Addr:       env("REPOHARBOR_ADDR", ":8080"),
		DataDir:    absData,
		DBPath:     filepath.Join(absData, "repoharbor.db"),
		ReposDir:   filepath.Join(absData, "repos"),
		LogsDir:    filepath.Join(absData, "logs"),
		SessionTTL: 7 * 24 * time.Hour,
		MaxRetries: 3,
	}
	for _, dir := range []string{cfg.DataDir, cfg.ReposDir, cfg.LogsDir} {
		if err := os.MkdirAll(dir, 0o755); err != nil {
			return Config{}, err
		}
	}
	return cfg, nil
}
