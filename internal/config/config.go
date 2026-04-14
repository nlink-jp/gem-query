// Package config manages gem-query configuration.
package config

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/BurntSushi/toml"
)

// Config holds all gem-query configuration.
type Config struct {
	GCP   GCPConfig   `toml:"gcp"`
	Model ModelConfig `toml:"model"`
	Tools ToolsConfig `toml:"tools"`
}

// ToolsConfig holds paths to external tool binaries.
type ToolsConfig struct {
	JvizPath string `toml:"jviz_path"`
}

// GCPConfig holds Google Cloud settings.
type GCPConfig struct {
	Project  string `toml:"project"`
	Location string `toml:"location"`
}

// ModelConfig holds model settings.
type ModelConfig struct {
	Name string `toml:"name"`
}

// Load reads config from the given path, with env var overrides.
// If path is empty, tries the default location (~/.config/gem-query/config.toml).
func Load(path string) (*Config, error) {
	cfg := &Config{
		GCP: GCPConfig{
			Location: "us-central1",
		},
		Model: ModelConfig{
			Name: "gemini-2.5-flash",
		},
	}

	if path == "" {
		home, err := os.UserHomeDir()
		if err == nil {
			path = filepath.Join(home, ".config", "gem-query", "config.toml")
		}
	}

	if path != "" {
		if _, err := os.Stat(path); err == nil {
			if _, err := toml.DecodeFile(path, cfg); err != nil {
				return nil, fmt.Errorf("parse config %s: %w", path, err)
			}
		}
	}

	// Env overrides (tool-specific > generic)
	if v := os.Getenv("GEMQUERY_PROJECT"); v != "" {
		cfg.GCP.Project = v
	} else if v := os.Getenv("GOOGLE_CLOUD_PROJECT"); v != "" {
		cfg.GCP.Project = v
	}
	if v := os.Getenv("GEMQUERY_LOCATION"); v != "" {
		cfg.GCP.Location = v
	} else if v := os.Getenv("GOOGLE_CLOUD_LOCATION"); v != "" {
		cfg.GCP.Location = v
	}
	if v := os.Getenv("GEMQUERY_MODEL"); v != "" {
		cfg.Model.Name = v
	}
	if v := os.Getenv("GEMQUERY_JVIZ_PATH"); v != "" {
		cfg.Tools.JvizPath = v
	}

	if cfg.GCP.Project == "" {
		return nil, fmt.Errorf("GCP project is required: set gcp.project in config or GOOGLE_CLOUD_PROJECT env var")
	}

	return cfg, nil
}

// ApplyFlags overrides config values with CLI flag values.
func (c *Config) ApplyFlags(model, jvizPath string) {
	if model != "" {
		c.Model.Name = model
	}
	if jvizPath != "" {
		c.Tools.JvizPath = jvizPath
	}
}
