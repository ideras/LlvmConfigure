package tools

import (
	"fmt"
	"os/exec"
	"strings"

	"llvm-configure/config"
	"llvm-configure/ui"
)

// LLVMTools holds the paths to LLVM tools.
type LLVMTools struct {
	LlvmAs string
	Llc    string
	Lld    string
}

type llvmToolDefinition struct {
	name       string
	configured string
	command    string
}

// commandExists checks if a command exists in PATH.
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
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

func missingTools(llvmAsFound, llcFound, lldFound bool) error {
	var missing []string
	if !llvmAsFound {
		missing = append(missing, "LLVM assembler")
	}
	if !llcFound {
		missing = append(missing, "LLVM compiler")
	}
	if !lldFound {
		missing = append(missing, "LLVM linker")
	}
	if len(missing) == 0 {
		return nil
	}

	return fmt.Errorf("missing tools: %s", strings.Join(missing, ", "))
}

func resolveLLVMTools(definitions []llvmToolDefinition) (*LLVMTools, error) {
	llvmAs, llvmAsFound := resolveTool(definitions[0].configured, definitions[0].command)
	llc, llcFound := resolveTool(definitions[1].configured, definitions[1].command)
	lld, lldFound := resolveTool(definitions[2].configured, definitions[2].command)

	if err := missingTools(llvmAsFound, llcFound, lldFound); err != nil {
		return nil, err
	}

	return &LLVMTools{LlvmAs: llvmAs, Llc: llc, Lld: lld}, nil
}

func configuredToolDefinitions(cfg *config.Config) []llvmToolDefinition {
	return []llvmToolDefinition{
		{name: "LLVM assembler", configured: cfg.LLVM.LlvmAs, command: "llvm-as"},
		{name: "LLVM compiler", configured: cfg.LLVM.Llc, command: "llc"},
		{name: "LLVM linker", configured: cfg.LLVM.Lld, command: "lld"},
	}
}

// CheckLLVMTools checks configured tool paths and falls back to the system PATH.
func CheckLLVMTools(cfg *config.Config) (*LLVMTools, error) {
	definitions := configuredToolDefinitions(cfg)

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

	return resolveLLVMTools(definitions)
}

// ScanLLVMTools searches the system PATH for all required LLVM tools.
func ScanLLVMTools() (*LLVMTools, error) {
	return resolveLLVMTools([]llvmToolDefinition{
		{name: "LLVM assembler", command: "llvm-as"},
		{name: "LLVM compiler", command: "llc"},
		{name: "LLVM linker", command: "lld"},
	})
}

// FindLLVMTools finds LLVM tools, first checking config, then the system PATH.
func FindLLVMTools(cfg *config.Config) (*LLVMTools, error) {
	return resolveLLVMTools(configuredToolDefinitions(cfg))
}
