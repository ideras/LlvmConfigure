package config

import (
	"encoding/json"
	"fmt"
	"llvm-configure/ui"
	"os"
	"path/filepath"
)

// LLVMConfig holds the LLVM tool paths
type LLVMConfig struct {
	LlvmAs string `json:"llvm_as"`
	Llc    string `json:"llc"`
	Lld    string `json:"lld"`
}

// LibCConfig holds the C library configuration
type LibCConfig struct {
	UseMusl       bool   `json:"use_musl"`
	Path          string `json:"path"`
	DynLinkerPath string `json:"dyn_linker_path"`
}

// Config holds the configuration loaded from file
type Config struct {
	LLVM LLVMConfig `json:"llvm"`
	LibC LibCConfig `json:"libc"`
}

func configFilePath() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	return filepath.Join(homeDir, ".llvm-configure", "config.json"), nil
}

// LoadConfigFile loads configuration from ~/.llvm-configure/config.json.
func LoadConfigFile() (*Config, error) {
	configPath, err := configFilePath()
	if err != nil {
		return nil, err
	}

	config := &Config{}

	data, err := os.ReadFile(configPath)
	if err != nil {
		// File doesn't exist, return empty config
		if os.IsNotExist(err) {
			return config, nil
		}
		return nil, err
	}

	fmt.Printf("Loading configuration from %s%s%s\n", ui.ColorCyan, configPath, ui.ColorReset)

	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	return config, nil
}

// SaveConfigFile writes configuration to ~/.llvm-configure/config.json.
func SaveConfigFile(config *Config) error {
	configPath, err := configFilePath()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode config JSON: %w", err)
	}

	return os.WriteFile(configPath, append(data, '\n'), 0644)
}
