package config

import (
	"os"
	"path/filepath"
	"strconv"

	"gopkg.in/yaml.v3"
)

// Config holds tui-sdd-llm-local user and project settings.
type Config struct {
	Model      string `yaml:"model"`
	OllamaHost string `yaml:"ollama_host"`
	GPUPrefer  string `yaml:"gpu_prefer"` // amd, nvidia, auto
	Theme      string `yaml:"theme"`
	FastMode   bool   `yaml:"fast_mode"`
}

// Default returns sensible defaults.
func Default() Config {
	return Config{
		Model:      "qwen2.5-coder:3b",
		OllamaHost: "http://127.0.0.1:11434",
		GPUPrefer:  "amd",
		Theme:      "k9s",
		FastMode:   true,
	}
}

// Load merges defaults, ~/.tsll/config.yaml, and .tsllrc in cwd.
func Load() Config {
	cfg := Default()

	if home, err := os.UserHomeDir(); err == nil {
		_ = mergeFile(&cfg, filepath.Join(home, ".tsll", "config.yaml"))
	}
	_ = mergeFile(&cfg, ".tsllrc")

	if v := os.Getenv("OLLAMA_HOST"); v != "" {
		cfg.OllamaHost = v
	}
	if v := os.Getenv("TSLL_MODEL"); v != "" {
		cfg.Model = v
	}
	if v := os.Getenv("TSLL_FAST"); v != "" {
		if parsed, err := strconv.ParseBool(v); err == nil {
			cfg.FastMode = parsed
		}
	}
	return cfg
}

func mergeFile(cfg *Config, path string) error {
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(data, cfg)
}

// Save writes config to ~/.tsll/config.yaml.
func Save(cfg Config) error {
	home, err := os.UserHomeDir()
	if err != nil {
		return err
	}
	dir := filepath.Join(home, ".tsll")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(dir, "config.yaml"), data, 0o644)
}
