package tools

import (
	"fmt"
	"os/exec"
	"strings"

	"llvm-configure/config"
	"llvm-configure/ui"
)

// LLVMTools holds the paths to LLVM tools
type LLVMTools struct {
	LlvmAs string
	Llc    string
	Lld    string
}

// commandExists checks if a command exists in PATH
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// FindLLVMTools finds LLVM tools, first checking config, then system
func FindLLVMTools(cfg *config.Config) (*LLVMTools, error) {
	tools := &LLVMTools{}
	foundLlvmAs := false
	foundLlc := false
	foundLld := false

	// Check config file first
	if cfg.LLVM.LlvmAs != "" && commandExists(cfg.LLVM.LlvmAs) {
		tools.LlvmAs = cfg.LLVM.LlvmAs
		foundLlvmAs = true
		fmt.Printf("Using llvm-as from config: %s%s%s\n", ui.ColorCyan, cfg.LLVM.LlvmAs, ui.ColorReset)
	}

	if cfg.LLVM.Llc != "" && commandExists(cfg.LLVM.Llc) {
		tools.Llc = cfg.LLVM.Llc
		foundLlc = true
		fmt.Printf("Using llc from config: %s%s%s\n", ui.ColorCyan, cfg.LLVM.Llc, ui.ColorReset)
	}

	if cfg.LLVM.Lld != "" && commandExists(cfg.LLVM.Lld) {
		tools.Lld = cfg.LLVM.Lld
		foundLld = true
		fmt.Printf("Using lld from config: %s%s%s\n", ui.ColorCyan, cfg.LLVM.Lld, ui.ColorReset)
	}

	// Search system for missing tools
	for version := 20; version >= 10; version-- {
		if !foundLlvmAs {
			llvmAS := fmt.Sprintf("llvm-as-%d", version)
			if commandExists(llvmAS) {
				path, _ := exec.LookPath(llvmAS)
				tools.LlvmAs = llvmAS
				foundLlvmAs = true
				fmt.Printf("Found llvm assembler at %s%s%s\n", ui.ColorCyan, path, ui.ColorReset)
			}
		}

		if !foundLlc {
			llc := fmt.Sprintf("llc-%d", version)
			if commandExists(llc) {
				path, _ := exec.LookPath(llc)
				tools.Llc = llc
				foundLlc = true
				fmt.Printf("Found llvm compiler at %s%s%s\n", ui.ColorCyan, path, ui.ColorReset)
			}
		}

		if !foundLld {
			lld := fmt.Sprintf("lld-%d", version)
			if commandExists(lld) {
				path, _ := exec.LookPath(lld)
				tools.Lld = lld
				foundLld = true
				fmt.Printf("Found llvm linker at %s%s%s\n", ui.ColorCyan, path, ui.ColorReset)
			}
		}

		if foundLlvmAs && foundLlc && foundLld {
			break
		}
	}

	if !foundLlvmAs || !foundLlc || !foundLld {
		var missing []string
		if !foundLlvmAs {
			missing = append(missing, "LLVM assembler")
		}
		if !foundLlc {
			missing = append(missing, "LLVM compiler")
		}
		if !foundLld {
			missing = append(missing, "LLVM linker")
		}
		return nil, fmt.Errorf("missing tools: %s", strings.Join(missing, ", "))
	}

	return tools, nil
}
