package config

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"

	"gopkg.in/yaml.v3"
)

// Config holds server configuration.
type Config struct {
	ManualCommandExecution bool `yaml:"manual_command_execution"`
}

// DefaultConfig returns the default configuration.
func DefaultConfig() *Config {
	return &Config{
		ManualCommandExecution: false,
	}
}

// Load reads configuration from file. Returns default config if file doesn't exist.
func Load(path string) (*Config, error) {
	cfg := DefaultConfig()

	data, err := os.ReadFile(path)
	if err != nil {
		if os.IsNotExist(err) {
			return cfg, nil // File doesn't exist, use defaults
		}
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	if err := yaml.Unmarshal(data, cfg); err != nil {
		return nil, fmt.Errorf("failed to parse config file: %w", err)
	}

	return cfg, nil
}

// LoadFromFlags handles --config flag and default config path resolution.
// Returns the loaded config and any error encountered.
func LoadFromFlags() (*Config, error) {
	configPath := flag.String("config", "", "Path to config.yaml file")
	flag.Parse()

	// If --config is provided, use it
	if *configPath != "" {
		return Load(*configPath)
	}

	// Otherwise, look for config.yaml in the executable's directory
	execPath, err := os.Executable()
	if err != nil {
		return DefaultConfig(), nil // Can't determine exec dir, use defaults
	}

	defaultPath := filepath.Join(filepath.Dir(execPath), "config.yaml")
	return Load(defaultPath)
}

// Global config instance (set at startup)
var Active *Config

// Init loads configuration from flags/files. Must be called at startup.
func Init() error {
	cfg, err := LoadFromFlags()
	if err != nil {
		return err
	}
	Active = cfg
	return nil
}
