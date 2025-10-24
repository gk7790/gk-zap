package config

import (
	"fmt"
	"os"
	"path/filepath"

	m "github.com/gk7790/gk-zap/pkg/config/model"
	"gopkg.in/yaml.v3"
)

func LoadYamlServerConfig(cfgPath string) (*m.ServerConfig, error) {
	ext := filepath.Ext(cfgPath)
	if ext != ".yaml" && ext != ".yml" {
		return nil, fmt.Errorf("only .yaml/.yml config files are supported, got: %s", ext)
	}
	file, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read config file: %w", err)
	}

	var cfg m.ServerConfig
	if err := yaml.Unmarshal(file, &cfg); err != nil {
		return nil, fmt.Errorf("invalid YAML config: %w", err)
	}

	// 调用 Complete() 补全默认项
	if err := cfg.Complete(); err != nil {
		return nil, fmt.Errorf("failed to complete config: %w", err)
	}

	return &cfg, nil
}
