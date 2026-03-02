package sources

import (
	"bufio"
	"fmt"
	"os"
	"path/filepath"
	"strings"
)

// ParseLLVMSourcesFile parses the LLVMSources.txt file
func ParseLLVMSourcesFile(srcFolder, sourcesFile string) ([]string, error) {
	sourcesPath := filepath.Join(srcFolder, sourcesFile)

	file, err := os.Open(sourcesPath)
	if err != nil {
		return nil, fmt.Errorf("%s not found in %s", sourcesFile, srcFolder)
	}
	defer file.Close()

	var sourceFiles []string
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := strings.TrimSpace(scanner.Text())

		// Skip empty lines and comments
		if line == "" || strings.HasPrefix(line, "#") {
			continue
		}

		var fullPath string
		if filepath.IsAbs(line) {
			fullPath = line
		} else {
			fullPath = filepath.Join(srcFolder, line)
		}

		// Validate file exists
		if _, err := os.Stat(fullPath); os.IsNotExist(err) {
			return nil, fmt.Errorf("file %s listed in %s:%d does not exist", fullPath, sourcesFile, lineNum)
		}

		// Validate .ll extension
		if !strings.HasSuffix(fullPath, ".ll") {
			return nil, fmt.Errorf("file %s listed in %s:%d is not a .ll file", fullPath, sourcesFile, lineNum)
		}

		sourceFiles = append(sourceFiles, fullPath)
	}

	return sourceFiles, scanner.Err()
}

// CreateLLVMSourcesFile creates the LLVMSources.txt file
func CreateLLVMSourcesFile(srcFolder string, llvmFiles []string, sourcesFile string) error {
	sourcesPath := filepath.Join(srcFolder, sourcesFile)

	file, err := os.Create(sourcesPath)
	if err != nil {
		return err
	}
	defer file.Close()

	file.WriteString("# LLVM Sources Configuration\n")
	file.WriteString("# One source file per line\n")
	file.WriteString("# Use absolute paths or relative paths (relative to this file's directory)\n")
	file.WriteString("#\n\n")

	for _, llvmFile := range llvmFiles {
		relPath, err := filepath.Rel(srcFolder, llvmFile)
		if err != nil || strings.HasPrefix(relPath, "..") {
			// Use absolute path if relative path calculation fails or goes outside src folder
			file.WriteString(llvmFile + "\n")
		} else {
			file.WriteString(relPath + "\n")
		}
	}

	return nil
}
