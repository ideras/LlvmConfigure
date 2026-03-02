package system

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// FindLibC finds the libc directories (GNU and/or MUSL)
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
		if line != "" && !strings.Contains(line, "musl") {
			libcPaths["gnu"] = filepath.Dir(line)
		} else if line != "" && strings.Contains(line, "musl") {
			libcPaths["musl"] = filepath.Dir(line)
		}
	}

	if len(libcPaths) > 0 {
		return libcPaths, nil
	}

	return nil, fmt.Errorf("GNU libc not found")
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

// GetDynamicLinkerPath gets the dynamic linker path
func GetDynamicLinkerPath() (string, error) {
	cmd := exec.Command("ldd", "/usr/bin/env")
	output, err := cmd.Output()
	if err != nil {
		return "", err
	}

	lines := strings.SplitSeq(string(output), "\n")
	for line := range lines {
		if strings.Contains(line, "ld-linux-x86-64.so.2") {
			fields := strings.Fields(line)
			if len(fields) > 0 {
				return fields[0], nil
			}
		}
	}

	return "", fmt.Errorf("dynamic linker not found")
}
