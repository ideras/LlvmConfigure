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

// LoadConfigFile loads configuration from ~/.llvm-configure/config.json
func LoadConfigFile() (*Config, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return nil, err
	}

	configPath := filepath.Join(homeDir, ".llvm-configure", "config.json")
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

	if config.LLVM.LlvmAs != "" {
		fmt.Printf("Found LLVM_AS in config: %s%s%s\n", ui.ColorCyan, config.LLVM.LlvmAs, ui.ColorReset)
	}
	if config.LLVM.Llc != "" {
		fmt.Printf("Found LLC in config: %s%s%s\n", ui.ColorCyan, config.LLVM.Llc, ui.ColorReset)
	}
	if config.LLVM.Lld != "" {
		fmt.Printf("Found LLD in config: %s%s%s\n", ui.ColorCyan, config.LLVM.Lld, ui.ColorReset)
	}
	if config.LibC.Path != "" {
		fmt.Printf("Found LIBC path in config: %s%s%s\n", ui.ColorCyan, config.LibC.Path, ui.ColorReset)
	}
	if config.LibC.DynLinkerPath != "" {
		fmt.Printf("Found dynamic linker in config: %s%s%s\n", ui.ColorCyan, config.LibC.DynLinkerPath, ui.ColorReset)
	}
	if config.LibC.UseMusl {
		fmt.Printf("Config specifies MUSL C library\n")
	}

	return config, nil
}
