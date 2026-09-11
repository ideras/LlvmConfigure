package system

import (
	"debug/elf"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

func findNativeGNUPath() string {
	for _, compiler := range []string{"cc", "gcc", "clang"} {
		path, err := exec.LookPath(compiler)
		if err != nil {
			continue
		}

		output, err := exec.Command(path, "-print-file-name=crti.o").Output()
		crtiPath := strings.TrimSpace(string(output))
		if err != nil || !filepath.IsAbs(crtiPath) || strings.Contains(crtiPath, "musl") {
			continue
		}
		if resolvedPath, err := filepath.EvalSymlinks(crtiPath); err == nil {
			crtiPath = resolvedPath
		}
		if _, err := os.Stat(crtiPath); err == nil {
			return filepath.Dir(crtiPath)
		}
	}

	return ""
}

// FindLibC finds the libc directories (GNU and/or MUSL).
func FindLibC() (map[string]string, error) {
	cmd := exec.Command("find", "/usr", "-name", "crti.o")
	cmd.Stderr = nil // Discard stderr (permission errors)

	output, err := cmd.Output()
	if err != nil && len(output) == 0 {
		return nil, err
	}

	lines := strings.SplitSeq(strings.TrimSpace(string(output)), "\n")
	libcPaths := make(map[string]string)
	for line := range lines {
		if line == "" {
			continue
		}

		kind := "gnu"
		if strings.Contains(strings.ToLower(line), "musl") {
			kind = "musl"
		}
		if _, found := libcPaths[kind]; !found {
			libcPaths[kind] = filepath.Dir(line)
		}
	}

	// Prefer the host compiler's libc over a cross-compilation sysroot.
	if nativeGNUPath := findNativeGNUPath(); nativeGNUPath != "" {
		libcPaths["gnu"] = nativeGNUPath
	}

	if len(libcPaths) > 0 {
		return libcPaths, nil
	}

	return nil, fmt.Errorf("standard libc not found")
}

// CheckObjectFiles checks if required object files exist in the libc path
func CheckObjectFiles(libcPath string) error {
	objFiles := []string{"crt1.o", "crti.o", "crtn.o"}

	for _, obj := range objFiles {
		objPath := filepath.Join(libcPath, obj)
		if _, err := os.Stat(objPath); os.IsNotExist(err) {
			return fmt.Errorf("object file %s not found", obj)
		}
	}

	return nil
}

// GetDynamicLinkerPath gets the dynamic linker path from the ELF interpreter.
func GetDynamicLinkerPath() (string, error) {
	executable, err := elf.Open("/usr/bin/env")
	if err != nil {
		return "", err
	}
	defer executable.Close()

	for _, program := range executable.Progs {
		if program.Type != elf.PT_INTERP {
			continue
		}

		interpreter, err := io.ReadAll(program.Open())
		if err != nil {
			return "", err
		}
		path := strings.TrimRight(string(interpreter), "\x00")
		if path != "" {
			return path, nil
		}
	}

	return "", fmt.Errorf("dynamic linker not found")
}
