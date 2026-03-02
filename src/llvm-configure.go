package main

import (
	"bufio"
	"encoding/json"
	"flag"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

const VERSION = "0.2.0"

// Color codes
const (
	ColorRed     = "\033[31m"
	ColorGreen   = "\033[32m"
	ColorYellow  = "\033[33m"
	ColorBlue    = "\033[34m"
	ColorMagenta = "\033[35m"
	ColorCyan    = "\033[36m"
	ColorReset   = "\033[0m"
)

// LLVMTools holds the paths to LLVM tools
type LLVMTools struct {
	LlvmAs string
	Llc    string
	Lld    string
}

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

// loadConfigFile loads configuration from ~/.llvm-configure/config.json
func loadConfigFile() (*Config, error) {
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

	fmt.Printf("Loading configuration from %s%s%s\n", ColorCyan, configPath, ColorReset)

	if err := json.Unmarshal(data, config); err != nil {
		return nil, fmt.Errorf("failed to parse config JSON: %w", err)
	}

	if config.LLVM.LlvmAs != "" {
		fmt.Printf("Found LLVM_AS in config: %s%s%s\n", ColorCyan, config.LLVM.LlvmAs, ColorReset)
	}
	if config.LLVM.Llc != "" {
		fmt.Printf("Found LLC in config: %s%s%s\n", ColorCyan, config.LLVM.Llc, ColorReset)
	}
	if config.LLVM.Lld != "" {
		fmt.Printf("Found LLD in config: %s%s%s\n", ColorCyan, config.LLVM.Lld, ColorReset)
	}
	if config.LibC.Path != "" {
		fmt.Printf("Found LIBC path in config: %s%s%s\n", ColorCyan, config.LibC.Path, ColorReset)
	}
	if config.LibC.DynLinkerPath != "" {
		fmt.Printf("Found dynamic linker in config: %s%s%s\n", ColorCyan, config.LibC.DynLinkerPath, ColorReset)
	}
	if config.LibC.UseMusl {
		fmt.Printf("Config specifies MUSL C library\n")
	}

	return config, nil
}

// commandExists checks if a command exists in PATH
func commandExists(cmd string) bool {
	_, err := exec.LookPath(cmd)
	return err == nil
}

// findLLVMTools finds LLVM tools, first checking config, then system
func findLLVMTools(config *Config) (*LLVMTools, error) {
	tools := &LLVMTools{}
	foundLlvmAs := false
	foundLlc := false
	foundLld := false

	// Check config file first
	if config.LLVM.LlvmAs != "" && commandExists(config.LLVM.LlvmAs) {
		tools.LlvmAs = config.LLVM.LlvmAs
		foundLlvmAs = true
		fmt.Printf("Using llvm-as from config: %s%s%s\n", ColorCyan, config.LLVM.LlvmAs, ColorReset)
	}

	if config.LLVM.Llc != "" && commandExists(config.LLVM.Llc) {
		tools.Llc = config.LLVM.Llc
		foundLlc = true
		fmt.Printf("Using llc from config: %s%s%s\n", ColorCyan, config.LLVM.Llc, ColorReset)
	}

	if config.LLVM.Lld != "" && commandExists(config.LLVM.Lld) {
		tools.Lld = config.LLVM.Lld
		foundLld = true
		fmt.Printf("Using lld from config: %s%s%s\n", ColorCyan, config.LLVM.Lld, ColorReset)
	}

	// Search system for missing tools
	for version := 20; version >= 10; version-- {
		if !foundLlvmAs {
			llvmAS := fmt.Sprintf("llvm-as-%d", version)
			if commandExists(llvmAS) {
				path, _ := exec.LookPath(llvmAS)
				tools.LlvmAs = llvmAS
				foundLlvmAs = true
				fmt.Printf("Found llvm assembler at %s%s%s\n", ColorCyan, path, ColorReset)
			}
		}

		if !foundLlc {
			llc := fmt.Sprintf("llc-%d", version)
			if commandExists(llc) {
				path, _ := exec.LookPath(llc)
				tools.Llc = llc
				foundLlc = true
				fmt.Printf("Found llvm compiler at %s%s%s\n", ColorCyan, path, ColorReset)
			}
		}

		if !foundLld {
			lld := fmt.Sprintf("lld-%d", version)
			if commandExists(lld) {
				path, _ := exec.LookPath(lld)
				tools.Lld = lld
				foundLld = true
				fmt.Printf("Found llvm linker at %s%s%s\n", ColorCyan, path, ColorReset)
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

// findLibC finds the GNU libc directory
func findLibC() (map[string]string, error) {
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

// checkObjectFiles checks if required object files exist
func checkObjectFiles(libcPath string) error {
	objFiles := []string{"crt1.o", "crti.o", "crtn.o"}

	for _, obj := range objFiles {
		objPath := filepath.Join(libcPath, obj)
		if _, err := os.Stat(objPath); os.IsNotExist(err) {
			return fmt.Errorf("object file %s not found", obj)
		}
	}

	return nil
}

// getDynamicLinkerPath gets the dynamic linker path
func getDynamicLinkerPath() (string, error) {
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

// parseLLVMSourcesFile parses the LLVMSources.txt file
func parseLLVMSourcesFile(srcFolder, sourcesFile string) ([]string, error) {
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

// createLLVMSourcesFile creates the LLVMSources.txt file
func createLLVMSourcesFile(srcFolder string, llvmFiles []string, sourcesFile string) error {
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

// buildMakefile generates the Makefile
func buildMakefile(buildFolder, srcFolder string, useMusl bool, tools *LLVMTools, asmFiles []string, libcDir, dynLinkerPath, exeName string) error {
	var ldFlags string
	if useMusl {
		ldFlags = "-static -nostdlib"
	} else {
		ldFlags = "-dynamic-linker $(DYNAMIC_LINKER)"
	}

	// Generate relative paths from build folder
	var relSrcFiles []string
	for _, asmFile := range asmFiles {
		relPath, err := filepath.Rel(buildFolder, asmFile)
		if err != nil {
			relPath = asmFile
		}
		relSrcFiles = append(relSrcFiles, relPath)
	}

	// Generate file lists
	srcFiles := strings.Join(relSrcFiles, " ")
	var bcFiles []string
	var objFiles []string
	for _, asmFile := range asmFiles {
		base := strings.TrimSuffix(filepath.Base(asmFile), ".ll")
		bcFiles = append(bcFiles, base+".bc")
		objFiles = append(objFiles, base+".o")
	}

	if exeName == "" {
		exeName = strings.TrimSuffix(filepath.Base(asmFiles[0]), ".ll")
	}

	makefilePath := filepath.Join(buildFolder, "Makefile")
	file, err := os.Create(makefilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	// Get relative path from build to src folder
	//relSrcFolder, _ := filepath.Rel(buildFolder, srcFolder)

	// Get absolute path to this executable
	exePath, _ := os.Executable()

	fmt.Fprintf(file, "# LLVM tools and filenames\n")
	fmt.Fprintf(file, "LLVM_AS := %s\n", tools.LlvmAs)
	fmt.Fprintf(file, "LLC     := %s\n", tools.Llc)
	fmt.Fprintf(file, "LLD     := %s\n", tools.Lld)
	fmt.Fprintf(file, "\n")
	fmt.Fprintf(file, "# Source files\n")
	fmt.Fprintf(file, "SRCS     := %s\n", srcFiles)
	fmt.Fprintf(file, "BITCODES := %s\n", strings.Join(bcFiles, " "))
	fmt.Fprintf(file, "OBJS     := %s\n", strings.Join(objFiles, " "))
	fmt.Fprintf(file, "EXE      := %s\n", exeName)
	fmt.Fprintf(file, "\n")
	fmt.Fprintf(file, "# System dynamic linker\n")
	fmt.Fprintf(file, "DYNAMIC_LINKER := %s\n", dynLinkerPath)
	fmt.Fprintf(file, "LIBC_DIR := %s\n", libcDir)
	fmt.Fprintf(file, "\n")
	fmt.Fprintf(file, ".PHONY: all clean rebuild configure\n")
	fmt.Fprintf(file, "\n")
	fmt.Fprintf(file, "all: $(EXE)\n")
	fmt.Fprintf(file, "\n")
	fmt.Fprintf(file, "# Pattern rule: Generate bitcode from LLVM IR files\n")
	fmt.Fprintf(file, "%%.bc: %%.ll\n")
	fmt.Fprintf(file, "\t$(LLVM_AS) $< -o $@\n")
	fmt.Fprintf(file, "\n")
	fmt.Fprintf(file, "# Pattern rule: Generate object files from bitcode\n")
	fmt.Fprintf(file, "%%.o: %%.bc\n")
	fmt.Fprintf(file, "\t$(LLC) $< -filetype=obj -o $@\n")
	fmt.Fprintf(file, "\n")
	fmt.Fprintf(file, "# Link all object files into an executable\n")
	fmt.Fprintf(file, "$(EXE): $(OBJS)\n")
	fmt.Fprintf(file, "\t$(LLD) -flavor gnu \\\n")
	fmt.Fprintf(file, "\t\t-o $(EXE) \\\n")
	fmt.Fprintf(file, "\t\t%s \\\n", ldFlags)
	fmt.Fprintf(file, "\t\t-L $(LIBC_DIR) \\\n")
	fmt.Fprintf(file, "\t\t$(LIBC_DIR)/crt1.o \\\n")
	fmt.Fprintf(file, "\t\t$(LIBC_DIR)/crti.o \\\n")
	fmt.Fprintf(file, "\t\t$(OBJS) -lc \\\n")
	fmt.Fprintf(file, "\t\t$(LIBC_DIR)/crtn.o\n")
	fmt.Fprintf(file, "\n")
	fmt.Fprintf(file, "clean:\n")
	fmt.Fprintf(file, "\trm -f $(BITCODES) $(OBJS) $(EXE)\n")
	fmt.Fprintf(file, "\n")
	fmt.Fprintf(file, "rebuild: clean all\n")
	fmt.Fprintf(file, "\n")
	fmt.Fprintf(file, "# Reconfigure using LLVMSources.txt\n")
	fmt.Fprintf(file, "configure:\n")
	fmt.Fprintf(file, "\t@echo \"Re-running configuration from LLVMSources.txt...\"\n")
	fmt.Fprintf(file, "\t@%s -B %s -S %s\n", exePath, buildFolder, srcFolder)
	fmt.Fprintf(file, "\n")
	fmt.Fprintf(file, "# Individual bitcode targets (for explicit building)")

	for i, asmFile := range asmFiles {
		base := strings.TrimSuffix(filepath.Base(asmFile), ".ll")
		fmt.Fprintf(file, "\n%s.bc: %s", base, relSrcFiles[i])
	}

	fmt.Fprintf(file, "\n")

	return nil
}

func main() {
	fmt.Printf("%sllvm_configure%s version %s%s%s\n", ColorBlue, ColorReset, ColorCyan, VERSION, ColorReset)
	fmt.Println("----------------------------")

	// Define flags
	buildFolder := flag.String("B", "", "Build folder path (required)")
	srcFolder := flag.String("S", "", "Source folder path (required)")
	withMusl := flag.Bool("with-musl", false, "Use MUSL C library instead of GNU libc")
	exeName := flag.String("exe-name", "", "Executable name (default: first source file basename)")
	sourcesFile := flag.String("sources-file", "LLVMSources.txt", "Source list file name")

	flag.Parse()

	// Validate required flags
	if *buildFolder == "" || *srcFolder == "" {
		fmt.Printf("%sError:%s Both -B and -S are required\n", ColorRed, ColorReset)
		flag.Usage()
		os.Exit(1)
	}

	// Get remaining arguments as LLVM files
	llvmFiles := flag.Args()

	// Make srcFolder absolute
	if !filepath.IsAbs(*srcFolder) {
		absSrc, err := filepath.Abs(*srcFolder)
		if err != nil {
			fmt.Printf("%sError:%s Cannot determine absolute path: %v\n", ColorRed, ColorReset, err)
			os.Exit(1)
		}
		*srcFolder = absSrc
	}

	// Load configuration
	config, err := loadConfigFile()
	if err != nil {
		fmt.Printf("%sWarning:%s Could not load config file: %v\n", ColorYellow, ColorReset, err)
		config = &Config{}
	}

	// Create source folder if it doesn't exist
	if _, err := os.Stat(*srcFolder); os.IsNotExist(err) {
		os.MkdirAll(*srcFolder, 0755)
		fmt.Printf("Created source folder: %s%s%s\n", ColorCyan, *srcFolder, ColorReset)
	}

	// Handle LLVM source files
	if len(llvmFiles) > 0 {
		var validatedFiles []string
		for _, filePath := range llvmFiles {
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				fmt.Printf("%sError:%s File %s%s%s does not exist\n", ColorRed, ColorReset, ColorCyan, filePath, ColorReset)
				os.Exit(1)
			}

			if !strings.HasSuffix(filePath, ".ll") {
				fmt.Printf("%sError:%s File %s%s%s is not a .ll file\n", ColorRed, ColorReset, ColorCyan, filePath, ColorReset)
				os.Exit(1)
			}

			absPath, _ := filepath.Abs(filePath)
			validatedFiles = append(validatedFiles, absPath)
		}

		fmt.Printf("Creating %s with %d LLVM assembly file(s):\n", *sourcesFile, len(validatedFiles))
		for _, f := range validatedFiles {
			fmt.Printf("  - %s%s%s\n", ColorCyan, filepath.Base(f), ColorReset)
		}

		if err := createLLVMSourcesFile(*srcFolder, validatedFiles, *sourcesFile); err != nil {
			fmt.Printf("%sError:%s Failed to create sources file: %v\n", ColorRed, ColorReset, err)
			os.Exit(1)
		}
		fmt.Printf("Created %s%s%s in %s%s%s\n", ColorCyan, *sourcesFile, ColorReset, ColorCyan, *srcFolder, ColorReset)
	}

	// Parse LLVMSources.txt
	sourceFiles, err := parseLLVMSourcesFile(*srcFolder, *sourcesFile)
	if err != nil {
		if len(llvmFiles) == 0 {
			fmt.Printf("%sError:%s No valid source files found.\n", ColorRed, ColorReset)
			fmt.Printf("Either provide .ll files as arguments or ensure %s exists in %s\n", *sourcesFile, *srcFolder)
		}
		fmt.Printf("%sError:%s %v\n", ColorRed, ColorReset, err)
		os.Exit(1)
	}

	fmt.Printf("Found %d source file(s) from %s%s%s:\n", len(sourceFiles), ColorCyan, *sourcesFile, ColorReset)
	for _, f := range sourceFiles {
		fmt.Printf("  - %s%s%s (%s)\n", ColorCyan, filepath.Base(f), ColorReset, f)
	}

	// Use MUSL from config if not specified on command line
	useMusl := *withMusl || config.LibC.UseMusl

	// Find libc
	var libcDir string
	var dynLinkerPath string

	if config.LibC.Path != "" {
		// Use libc path from config
		libcDir = config.LibC.Path
		fmt.Printf("Using C library path from config: %s%s%s\n", ColorCyan, libcDir, ColorReset)
	} else {
		// Auto-detect libc
		libcPaths, err := findLibC()
		if err != nil {
			fmt.Printf("%sError:%s Cannot find C library: %v\n", ColorRed, ColorReset, err)
			os.Exit(1)
		}

		if useMusl {
			libcDirMusl, ok := libcPaths["musl"]
			if !ok {
				fmt.Printf("%sError:%s Cannot find MUSL C library\n", ColorRed, ColorReset)
				os.Exit(1)
			}
			libcDir = libcDirMusl
			fmt.Printf("Found MUSL C library at %s%s%s\n", ColorCyan, libcDir, ColorReset)
		} else {
			libcDirGnu, ok := libcPaths["gnu"]
			if !ok {
				fmt.Printf("%sError:%s Cannot find standard C library\n", ColorRed, ColorReset)
				os.Exit(1)
			}
			libcDir = libcDirGnu
			fmt.Printf("Found standard C library at %s%s%s\n", ColorCyan, libcDir, ColorReset)
		}
	}

	if !useMusl {
		if config.LibC.DynLinkerPath != "" {
			dynLinkerPath = config.LibC.DynLinkerPath
			fmt.Printf("Using dynamic linker from config: %s%s%s\n", ColorCyan, dynLinkerPath, ColorReset)
		} else {
			dynLinkerPath, err = getDynamicLinkerPath()
			if err != nil {
				fmt.Printf("%sError:%s Cannot find linux dynamic linker\n", ColorRed, ColorReset)
				os.Exit(1)
			}
			fmt.Printf("Found Linux dynamic linker at %s%s%s\n", ColorCyan, dynLinkerPath, ColorReset)
		}
	}

	// Check object files
	if err := checkObjectFiles(libcDir); err != nil {
		fmt.Printf("%sError:%s %v\n", ColorRed, ColorReset, err)
		os.Exit(1)
	}

	// Find LLVM tools
	tools, err := findLLVMTools(config)
	if err != nil {
		fmt.Printf("%sError:%s %v\n", ColorRed, ColorReset, err)
		os.Exit(1)
	}

	// Check if build folder exists
	if _, err := os.Stat(*buildFolder); !os.IsNotExist(err) {
		fmt.Printf("%sError:%s Target directory %s%s%s already exists\n", ColorRed, ColorReset, ColorBlue, *buildFolder, ColorReset)
		os.Exit(1)
	}

	// Create build folder
	if err := os.MkdirAll(*buildFolder, 0755); err != nil {
		fmt.Printf("%sError:%s Cannot create build folder: %v\n", ColorRed, ColorReset, err)
		os.Exit(1)
	}

	fmt.Printf("Generating Makefile in folder %s%s%s ", ColorCyan, *buildFolder, ColorReset)

	// Build makefile
	if err := buildMakefile(*buildFolder, *srcFolder, useMusl, tools, sourceFiles, libcDir, dynLinkerPath, *exeName); err != nil {
		fmt.Printf("%s%sError:%s Failed to create Makefile: %v\n", ColorRed, ColorReset, ColorReset, err)
		os.Exit(1)
	}

	fmt.Printf("%sdone%s ...\n", ColorGreen, ColorReset)
	fmt.Printf("Run %smake%s in %s%s%s to build the project.\n", ColorCyan, ColorReset, ColorCyan, *buildFolder, ColorReset)
	fmt.Printf("Edit %s%s%s to modify source files and run %smake configure%s to regenerate.\n",
		ColorCyan, filepath.Join(*srcFolder, *sourcesFile), ColorReset, ColorCyan, ColorReset)
}
