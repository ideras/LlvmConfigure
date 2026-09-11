package tools

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"llvm-configure/config"
	"llvm-configure/platform"
	"llvm-configure/ui"
)

// LLVMTools holds the paths to LLVM tools.
//
// Lld is the Linux linker driver; Clang is the Darwin linker driver.
// Each field is only populated for the platform that requires it:
//
//	Linux  requires llvm-as, llc, lld
//	Darwin requires llvm-as, llc, clang
type LLVMTools struct {
	LlvmAs string
	Llc    string
	Lld    string // Linux linker driver; empty on Darwin
	Clang  string // Darwin linker driver; empty on Linux
}

type llvmToolDefinition struct {
	name       string
	configured string
	command    string
}

// linkerCommand returns the linker driver command required by a platform.
func linkerCommand(kind platform.PlatformKind) (string, error) {
	switch kind {
	case platform.PlatformLinux:
		return "lld", nil
	case platform.PlatformDarwin:
		return "clang", nil
	default:
		return "", fmt.Errorf("unsupported platform %q", string(kind))
	}
}

// linkerConfiguredPath returns the configured linker path for a platform.
func linkerConfiguredPath(cfg *config.Config, kind platform.PlatformKind) (string, error) {
	switch kind {
	case platform.PlatformLinux:
		return cfg.LLVM.Lld, nil
	case platform.PlatformDarwin:
		return cfg.LLVM.Clang, nil
	default:
		return "", fmt.Errorf("unsupported platform %q", string(kind))
	}
}

// commandExists checks if a command exists in PATH.
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// isExecutableFile reports whether path exists and is a regular, executable
// file. Directory entries and non-executable files are rejected.
func isExecutableFile(path string) bool {
	if path == "" {
		return false
	}
	info, err := os.Stat(path)
	if err != nil || info.IsDir() {
		return false
	}
	return info.Mode()&0111 != 0
}

func findSystemTool(command string) (string, bool) {
	for version := 20; version >= 10; version-- {
		if path, err := exec.LookPath(fmt.Sprintf("%s-%d", command, version)); err == nil {
			return path, true
		}
	}

	path, err := exec.LookPath(command)
	return path, err == nil
}

func resolveTool(configured, command string) (string, bool) {
	if configured != "" && commandExists(configured) {
		path, _ := exec.LookPath(configured)
		return path, true
	}

	return findSystemTool(command)
}

func missingTools(found []bool, names []string) error {
	var missing []string
	for i, isFound := range found {
		if !isFound {
			missing = append(missing, names[i])
		}
	}
	if len(missing) == 0 {
		return nil
	}

	return fmt.Errorf("missing tools: %s", strings.Join(missing, ", "))
}

func resolveLLVMTools(definitions []llvmToolDefinition, kind platform.PlatformKind) (*LLVMTools, error) {
	results := make([]string, len(definitions))
	found := make([]bool, len(definitions))
	names := make([]string, len(definitions))

	for i, def := range definitions {
		path, ok := resolveTool(def.configured, def.command)
		results[i] = path
		found[i] = ok
		names[i] = def.name
	}

	if err := missingTools(found, names); err != nil {
		return nil, err
	}

	result := &LLVMTools{LlvmAs: results[0], Llc: results[1]}
	if err := applyLinker(result, results[2], kind); err != nil {
		return nil, err
	}
	return result, nil
}

// applyLinker stores the resolved linker driver in the platform-appropriate
// field (Lld on Linux, Clang on Darwin).
func applyLinker(t *LLVMTools, linkerPath string, kind platform.PlatformKind) error {
	switch kind {
	case platform.PlatformLinux:
		t.Lld = linkerPath
	case platform.PlatformDarwin:
		t.Clang = linkerPath
	default:
		return fmt.Errorf("unsupported platform %q", string(kind))
	}
	return nil
}

func configuredToolDefinitions(cfg *config.Config, kind platform.PlatformKind) ([]llvmToolDefinition, error) {
	linkerCmd, err := linkerCommand(kind)
	if err != nil {
		return nil, err
	}
	linkerPath, err := linkerConfiguredPath(cfg, kind)
	if err != nil {
		return nil, err
	}

	return []llvmToolDefinition{
		{name: "LLVM assembler", configured: cfg.LLVM.LlvmAs, command: "llvm-as"},
		{name: "LLVM compiler", configured: cfg.LLVM.Llc, command: "llc"},
		{name: "Linker driver", configured: linkerPath, command: linkerCmd},
	}, nil
}

// CheckLLVMTools checks configured tool paths and falls back to the system PATH.
func CheckLLVMTools(cfg *config.Config, kind platform.PlatformKind) (*LLVMTools, error) {
	definitions, err := configuredToolDefinitions(cfg, kind)
	if err != nil {
		return nil, err
	}

	for _, tool := range definitions {
		if tool.configured != "" {
			fmt.Printf("Checking %s at %s%s%s ... ", tool.name, ui.ColorCyan, tool.configured, ui.ColorReset)
			if commandExists(tool.configured) {
				fmt.Printf("%s[Found]%s\n", ui.ColorGreen, ui.ColorReset)
				continue
			}
			fmt.Printf("%s[Not Found]%s\n", ui.ColorRed, ui.ColorReset)
		}

		fmt.Printf("Checking %s in PATH ... ", tool.name)
		if path, found := findSystemTool(tool.command); found {
			fmt.Printf("%s[Found: %s]%s\n", ui.ColorGreen, path, ui.ColorReset)
		} else {
			fmt.Printf("%s[Not Found]%s\n", ui.ColorRed, ui.ColorReset)
		}
	}

	return resolveLLVMTools(definitions, kind)
}

// ScanLLVMTools searches the system PATH for all required LLVM tools.
func ScanLLVMTools(kind platform.PlatformKind) (*LLVMTools, error) {
	return resolveLLVMTools([]llvmToolDefinition{
		{name: "LLVM assembler", command: "llvm-as"},
		{name: "LLVM compiler", command: "llc"},
		{name: "Linker driver", command: mustLinkerCommand(kind)},
	}, kind)
}

func mustLinkerCommand(kind platform.PlatformKind) string {
	cmd, err := linkerCommand(kind)
	if err != nil {
		return ""
	}
	return cmd
}

// FindLLVMTools finds LLVM tools, first checking config, then the system PATH.
func FindLLVMTools(cfg *config.Config, kind platform.PlatformKind) (*LLVMTools, error) {
	definitions, err := configuredToolDefinitions(cfg, kind)
	if err != nil {
		return nil, err
	}
	return resolveLLVMTools(definitions, kind)
}
