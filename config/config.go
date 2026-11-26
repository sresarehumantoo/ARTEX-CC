package config

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
)

const engagementFile = ".artex.json"

type Config struct {
	Listen    string `json:"listen" toml:"listen"`
	PGDSN     string `json:"pg_dsn" toml:"pg_dsn"`
	ProxyAddr string `json:"proxy_addr,omitempty" toml:"proxy_addr"`
	DataDir   string `json:"data_dir" toml:"data_dir"`
	SkillDir  string `json:"skill_dir,omitempty" toml:"skill_dir"`
	PluginDir string `json:"plugin_dir,omitempty" toml:"plugin_dir"`
}

type EngagementConfig struct {
	TaskID        int64  `json:"task_id"`
	ExplorationID int64  `json:"exploration_id"`
	Goal          string `json:"goal,omitempty"`
	Description   string `json:"description,omitempty"`
}

func Default() Config {
	return Config{
		Listen:  "localhost:9400",
		DataDir: ".",
	}
}

func LoadEngagement(dataDir string) (EngagementConfig, error) {
	path := filepath.Join(dataDir, engagementFile)
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return EngagementConfig{}, nil
		}
		return EngagementConfig{}, err
	}
	var cfg EngagementConfig
	if err := json.Unmarshal(data, &cfg); err != nil {
		return EngagementConfig{}, err
	}
	return cfg, nil
}

func SaveEngagement(dataDir string, cfg EngagementConfig) error {
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	data = append(data, '\n')
	return os.WriteFile(filepath.Join(dataDir, engagementFile), data, 0o600)
}

// LoadOrDefault reads a config file at path. Both JSON (.json) and TOML (.toml)
// formats are supported; the file extension determines the parser. A missing
// file returns Default().
func LoadOrDefault(path string) (Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return Default(), nil
		}
		return Config{}, err
	}
	cfg := Default()
	if err := unmarshalConfig(path, data, &cfg); err != nil {
		return Config{}, err
	}
	return cfg, nil
}

func unmarshalConfig(path string, data []byte, cfg *Config) error {
	if strings.HasSuffix(path, ".toml") {
		return toml.Unmarshal(data, cfg)
	}
	return json.Unmarshal(data, cfg)
}
