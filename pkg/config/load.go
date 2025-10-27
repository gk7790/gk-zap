package config

import (
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	m1 "github.com/gk7790/gk-zap/pkg/config/model"
	"gopkg.in/yaml.v3"
)

type Values struct {
	Envs map[string]string
}

var glbEnvs map[string]string

func init() {
	glbEnvs = make(map[string]string)
	for _, env := range os.Environ() {
		pair := strings.SplitN(env, "=", 2)
		if len(pair) == 2 {
			glbEnvs[pair[0]] = pair[1]
		}
	}
}

func GetValues() *Values {
	return &Values{Envs: glbEnvs}
}

// RenderWithTemplate Render YAML template with environment values
func RenderWithTemplate(in []byte, values *Values) ([]byte, error) {
	tmpl, err := template.New("yaml").Parse(string(in))
	if err != nil {
		return nil, err
	}

	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, values); err != nil {
		return nil, err
	}
	return buf.Bytes(), nil
}

func LoadYamlServerConfig(cfgPath string) (*m1.ServerConfig, error) {
	ext := filepath.Ext(cfgPath)
	if ext != ".yaml" && ext != ".yml" {
		return nil, fmt.Errorf("only .yaml/.yml config files are supported, got: %s", ext)
	}
	file, err := os.ReadFile(cfgPath)
	if err != nil {
		return nil, fmt.Errorf("cannot read config file: %w", err)
	}

	var cfg m1.ServerConfig
	if err := yaml.Unmarshal(file, &cfg); err != nil {
		return nil, fmt.Errorf("invalid YAML config: %w", err)
	}

	// 调用 Complete() 补全默认项
	if err := cfg.Complete(); err != nil {
		return nil, fmt.Errorf("failed to complete config: %w", err)
	}

	return &cfg, nil
}

// LoadYAMLFile Load YAML file and render template
func LoadYAMLFile(path string) ([]byte, error) {
	b, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	return RenderWithTemplate(b, GetValues())
}

// LoadYAML Unmarshal YAML content into target struct
func LoadYAML(path string, out any) error {
	b, err := LoadYAMLFile(path)
	if err != nil {
		return err
	}
	return yaml.Unmarshal(b, out)
}
