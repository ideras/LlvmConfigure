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
	// Clang is the Darwin linker driver. It is empty on Linux builds,
	// where lld is used instead.
	Clang string `json:"clang"`
}

// TargetConfig holds the persisted LLVM target triple information.
//
// The triple is detected from the selected clang toolchain
// (clang -print-target-triple) and validated before every Darwin build.
// DeploymentTarget is a user-selected macOS compatibility policy; when it
// is empty the toolchain's detected/default macOS target is used.
type TargetConfig struct {
	Triple           string `json:"triple"`
	DeploymentTarget string `json:"deployment_target"`
	// DetectedBy records the clang binary that produced the stored triple.
	DetectedBy string `json:"detected_by"`
}

// LibCConfig holds the C library configuration.
// These fields are only meaningful on Linux; Darwin builds ignore them.
type LibCConfig struct {
	UseMusl       bool   `json:"use_musl"`
	Path          string `json:"path"`
	DynLinkerPath string `json:"dyn_linker_path"`
}

// Config holds the configuration loaded from file
type Config struct {
	LLVM   LLVMConfig   `json:"llvm"`
	Target TargetConfig `json:"target"`
	LibC   LibCConfig   `json:"libc"`
}

// configPathOverride, when set, replaces the default configuration file
// location. It exists so tests can exercise load/save without touching
// the real ~/.llvm-configure/config.json.
var configPathOverride string

// SetConfigPathOverride overrides the configuration file path.
// It must only be used by tests.
func SetConfigPathOverride(path string) { configPathOverride = path }

func configFilePath() (string, error) {
	if configPathOverride != "" {
		return configPathOverride, nil
	}

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

	return LoadConfigFrom(configPath)
}

// LoadConfigFrom loads configuration from an explicit path.
func LoadConfigFrom(configPath string) (*Config, error) {
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

	return SaveConfigTo(configPath, config)
}

// SaveConfigTo writes configuration to an explicit path.
func SaveConfigTo(configPath string, config *Config) error {
	if err := os.MkdirAll(filepath.Dir(configPath), 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(config, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode config JSON: %w", err)
	}

	return os.WriteFile(configPath, append(data, '\n'), 0644)
}
