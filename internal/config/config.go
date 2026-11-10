package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
)

const ConfigFileName = "config.json"

type Config struct {
	ClientID     string `json:"client_id"`
	ClientSecret string `json:"client_secret"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	AutoStart    bool   `json:"auto_start"`
}

// DefaultConfig returns default configuration values
func DefaultConfig() *Config {
	return &Config{
		ClientID:     "",
		ClientSecret: "",
		Host:         "127.0.0.1",
		Port:         8888,
		AutoStart:    false,
	}
}

// Load loads configuration from config.json, environment variables, or defaults
func Load() (*Config, error) {
	cfg := DefaultConfig()

	// Try reading config.json
	path := getConfigPath()
	if data, err := os.ReadFile(path); err == nil {
		if err := json.Unmarshal(data, cfg); err == nil {
			return cfg, nil
		}
	}

	// Fallback to environment variables if present
	if id := os.Getenv("CLIENT_ID"); id != "" {
		cfg.ClientID = id
	}
	if secret := os.Getenv("CLIENT_SECRET"); secret != "" {
		cfg.ClientSecret = secret
	}
	if host := os.Getenv("HOST"); host != "" {
		cfg.Host = host
	}
	if portStr := os.Getenv("PORT"); portStr != "" {
		if p, err := strconv.Atoi(portStr); err == nil && p > 0 {
			cfg.Port = p
		}
	}

	return cfg, nil
}

// Save saves configuration to config.json
func (c *Config) Save() error {
	path := getConfigPath()
	data, err := json.MarshalIndent(c, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

func getConfigPath() string {
	// First check current working directory
	if _, err := os.Stat(ConfigFileName); err == nil {
		return ConfigFileName
	}

	// Try executable directory
	if exe, err := os.Executable(); err == nil {
		exeDir := filepath.Dir(exe)
		candidate := filepath.Join(exeDir, ConfigFileName)
		if _, err := os.Stat(candidate); err == nil {
			return candidate
		}
	}

	return ConfigFileName
}
