package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"llvm-configure/config"
	"llvm-configure/makefile"
	"llvm-configure/sources"
	"llvm-configure/system"
	"llvm-configure/tools"
	"llvm-configure/ui"
)

func main() {
	fmt.Printf("%sllvm_configure%s version %s%s%s\n", ui.ColorBlue, ui.ColorReset, ui.ColorCyan, ui.VERSION, ui.ColorReset)
	fmt.Println("----------------------------")

	// Define flags
	buildFolder := flag.String("B", "", "Build folder path (required)")
	srcFolder := flag.String("S", "", "Source folder path (required)")
	withMusl := flag.Bool("with-musl", false, "Use MUSL C library instead of GNU libc")
	exeName := flag.String("exe-name", "", "Executable name (default: first source file basename)")
	sourcesFile := flag.String("sources-file", "LLVMSources.txt", "Source list file name")
	checkLLVM := flag.Bool("check-llvm", false, "Check whether required LLVM tools are installed")
	checkLibC := flag.Bool("check-libc", false, "Check whether GNU libc is installed")
	checkMusl := flag.Bool("check-musl", false, "Check whether musl libc is installed")
	scanLLVM := flag.Bool("scan-llvm", false, "Scan the system for LLVM tools and update the config file")
	scanLibC := flag.Bool("scan-libc", false, "Scan the system for GNU libc and update the config file")

	flag.Parse()

	if *checkLLVM || *checkLibC || *checkMusl || *scanLLVM || *scanLibC {
		cfg, err := config.LoadConfigFile()
		if err != nil {
			fmt.Printf("%sWarning:%s Could not load config file: %v\n", ui.ColorYellow, ui.ColorReset, err)
			cfg = &config.Config{}
		}

		exitCode := 0
		if *scanLLVM {
			llvmTools, err := tools.ScanLLVMTools()
			if err != nil {
				fmt.Printf("%sLLVM tool scan failed:%s %v\n", ui.ColorRed, ui.ColorReset, err)
				exitCode = 1
			} else {
				cfg.LLVM = config.LLVMConfig{
					LlvmAs: llvmTools.LlvmAs,
					Llc:    llvmTools.Llc,
					Lld:    llvmTools.Lld,
				}
				if err := config.SaveConfigFile(cfg); err != nil {
					fmt.Printf("%sFailed to update LLVM configuration:%s %v\n", ui.ColorRed, ui.ColorReset, err)
					exitCode = 1
				} else {
					fmt.Printf("%sUpdated LLVM tool paths in the configuration file.%s\n", ui.ColorGreen, ui.ColorReset)
				}
			}
		}

		if *scanLibC {
			libcPaths, err := system.FindLibC()
			libcPath, found := libcPaths["gnu"]
			if err != nil {
				fmt.Printf("%sGNU libc scan failed:%s %v\n", ui.ColorRed, ui.ColorReset, err)
				exitCode = 1
			} else if !found {
				fmt.Printf("%sGNU libc scan failed:%s GNU libc not found\n", ui.ColorRed, ui.ColorReset)
				exitCode = 1
			} else if err := system.CheckObjectFiles(libcPath); err != nil {
				fmt.Printf("%sGNU libc scan failed:%s %v\n", ui.ColorRed, ui.ColorReset, err)
				exitCode = 1
			} else {
				dynLinkerPath, err := system.GetDynamicLinkerPath()
				if err != nil {
					fmt.Printf("%sGNU libc scan failed:%s cannot find dynamic linker: %v\n", ui.ColorRed, ui.ColorReset, err)
					exitCode = 1
				} else {
					cfg.LibC = config.LibCConfig{
						Path:          libcPath,
						DynLinkerPath: dynLinkerPath,
					}
					if err := config.SaveConfigFile(cfg); err != nil {
						fmt.Printf("%sFailed to update GNU libc configuration:%s %v\n", ui.ColorRed, ui.ColorReset, err)
						exitCode = 1
					} else {
						fmt.Printf("%sUpdated GNU libc and dynamic linker paths in the configuration file.%s\n", ui.ColorGreen, ui.ColorReset)
					}
				}
			}
		}

		if *checkLLVM {
			if _, err := tools.CheckLLVMTools(cfg); err != nil {
				fmt.Printf("%sLLVM tools not found:%s %v\n", ui.ColorRed, ui.ColorReset, err)
				exitCode = 1
			} else {
				fmt.Printf("%sLLVM tools are installed.%s\n", ui.ColorGreen, ui.ColorReset)
			}
		}

		if *checkLibC {
			libcPaths, err := system.FindLibC()
			libcPath, found := libcPaths["gnu"]
			if err != nil || !found {
				fmt.Printf("%sGNU libc not found.%s\n", ui.ColorRed, ui.ColorReset)
				exitCode = 1
			} else if err := system.CheckObjectFiles(libcPath); err != nil {
				fmt.Printf("%sGNU libc is incomplete:%s %v\n", ui.ColorRed, ui.ColorReset, err)
				exitCode = 1
			} else {
				fmt.Printf("%sGNU libc is installed at %s%s%s\n", ui.ColorGreen, ui.ColorCyan, libcPath, ui.ColorReset)
			}
		}

		if *checkMusl {
			libcPaths, err := system.FindLibC()
			muslPath, found := libcPaths["musl"]
			if err != nil || !found {
				fmt.Printf("%smusl libc not found.%s\n", ui.ColorRed, ui.ColorReset)
				exitCode = 1
			} else {
				fmt.Printf("%smusl libc is installed at %s%s%s\n", ui.ColorGreen, ui.ColorCyan, muslPath, ui.ColorReset)
			}
		}

		os.Exit(exitCode)
	}

	// Validate required flags
	if *buildFolder == "" || *srcFolder == "" {
		fmt.Printf("%sError:%s Both -B and -S are required\n", ui.ColorRed, ui.ColorReset)
		flag.Usage()
		os.Exit(1)
	}

	// Get remaining arguments as LLVM files
	llvmFiles := flag.Args()

	// Make srcFolder absolute
	if !filepath.IsAbs(*srcFolder) {
		absSrc, err := filepath.Abs(*srcFolder)
		if err != nil {
			fmt.Printf("%sError:%s Cannot determine absolute path: %v\n", ui.ColorRed, ui.ColorReset, err)
			os.Exit(1)
		}
		*srcFolder = absSrc
	}

	// Load configuration
	cfg, err := config.LoadConfigFile()
	if err != nil {
		fmt.Printf("%sWarning:%s Could not load config file: %v\n", ui.ColorYellow, ui.ColorReset, err)
		cfg = &config.Config{}
	}

	// Create source folder if it doesn't exist
	if _, err := os.Stat(*srcFolder); os.IsNotExist(err) {
		os.MkdirAll(*srcFolder, 0755)
		fmt.Printf("Created source folder: %s%s%s\n", ui.ColorCyan, *srcFolder, ui.ColorReset)
	}

	// Handle LLVM source files
	if len(llvmFiles) > 0 {
		var validatedFiles []string
		for _, filePath := range llvmFiles {
			if _, err := os.Stat(filePath); os.IsNotExist(err) {
				fmt.Printf("%sError:%s File %s%s%s does not exist\n", ui.ColorRed, ui.ColorReset, ui.ColorCyan, filePath, ui.ColorReset)
				os.Exit(1)
			}

			if !strings.HasSuffix(filePath, ".ll") {
				fmt.Printf("%sError:%s File %s%s%s is not a .ll file\n", ui.ColorRed, ui.ColorReset, ui.ColorCyan, filePath, ui.ColorReset)
				os.Exit(1)
			}

			absPath, _ := filepath.Abs(filePath)
			validatedFiles = append(validatedFiles, absPath)
		}

		fmt.Printf("Creating %s with %d LLVM assembly file(s):\n", *sourcesFile, len(validatedFiles))
		for _, f := range validatedFiles {
			fmt.Printf("  - %s%s%s\n", ui.ColorCyan, filepath.Base(f), ui.ColorReset)
		}

		if err := sources.CreateLLVMSourcesFile(*srcFolder, validatedFiles, *sourcesFile); err != nil {
			fmt.Printf("%sError:%s Failed to create sources file: %v\n", ui.ColorRed, ui.ColorReset, err)
			os.Exit(1)
		}
		fmt.Printf("Created %s%s%s in %s%s%s\n", ui.ColorCyan, *sourcesFile, ui.ColorReset, ui.ColorCyan, *srcFolder, ui.ColorReset)
	}

	// Parse LLVMSources.txt
	sourceFiles, err := sources.ParseLLVMSourcesFile(*srcFolder, *sourcesFile)
	if err != nil {
		if len(llvmFiles) == 0 {
			fmt.Printf("%sError:%s No valid source files found.\n", ui.ColorRed, ui.ColorReset)
			fmt.Printf("Either provide .ll files as arguments or ensure %s exists in %s\n", *sourcesFile, *srcFolder)
		}
		fmt.Printf("%sError:%s %v\n", ui.ColorRed, ui.ColorReset, err)
		os.Exit(1)
	}

	fmt.Printf("Found %d source file(s) from %s%s%s:\n", len(sourceFiles), ui.ColorCyan, *sourcesFile, ui.ColorReset)
	for _, f := range sourceFiles {
		fmt.Printf("  - %s%s%s (%s)\n", ui.ColorCyan, filepath.Base(f), ui.ColorReset, f)
	}

	// Use MUSL from config if not specified on command line
	useMusl := *withMusl || cfg.LibC.UseMusl

	// Find libc
	var libcDir string
	var dynLinkerPath string

	if cfg.LibC.Path != "" {
		// Use libc path from config
		libcDir = cfg.LibC.Path
		fmt.Printf("Using C library path from config: %s%s%s\n", ui.ColorCyan, libcDir, ui.ColorReset)
	} else {
		// Auto-detect libc
		libcPaths, err := system.FindLibC()
		if err != nil {
			fmt.Printf("%sError:%s Cannot find C library: %v\n", ui.ColorRed, ui.ColorReset, err)
			os.Exit(1)
		}

		if useMusl {
			libcDirMusl, ok := libcPaths["musl"]
			if !ok {
				fmt.Printf("%sError:%s Cannot find MUSL C library\n", ui.ColorRed, ui.ColorReset)
				os.Exit(1)
			}
			libcDir = libcDirMusl
			fmt.Printf("Found MUSL C library at %s%s%s\n", ui.ColorCyan, libcDir, ui.ColorReset)
		} else {
			libcDirGnu, ok := libcPaths["gnu"]
			if !ok {
				fmt.Printf("%sError:%s Cannot find standard C library\n", ui.ColorRed, ui.ColorReset)
				os.Exit(1)
			}
			libcDir = libcDirGnu
			fmt.Printf("Found standard C library at %s%s%s\n", ui.ColorCyan, libcDir, ui.ColorReset)
		}
	}

	if !useMusl {
		if cfg.LibC.DynLinkerPath != "" {
			dynLinkerPath = cfg.LibC.DynLinkerPath
			fmt.Printf("Using dynamic linker from config: %s%s%s\n", ui.ColorCyan, dynLinkerPath, ui.ColorReset)
		} else {
			dynLinkerPath, err = system.GetDynamicLinkerPath()
			if err != nil {
				fmt.Printf("%sError:%s Cannot find linux dynamic linker\n", ui.ColorRed, ui.ColorReset)
				os.Exit(1)
			}
			fmt.Printf("Found Linux dynamic linker at %s%s%s\n", ui.ColorCyan, dynLinkerPath, ui.ColorReset)
		}
	}

	// Check object files
	if err := system.CheckObjectFiles(libcDir); err != nil {
		fmt.Printf("%sError:%s %v\n", ui.ColorRed, ui.ColorReset, err)
		os.Exit(1)
	}

	// Find LLVM tools
	llvmTools, err := tools.FindLLVMTools(cfg)
	if err != nil {
		fmt.Printf("%sError:%s %v\n", ui.ColorRed, ui.ColorReset, err)
		os.Exit(1)
	}

	// Check if build folder exists
	if _, err := os.Stat(*buildFolder); !os.IsNotExist(err) {
		fmt.Printf("%sError:%s Target directory %s%s%s already exists\n", ui.ColorRed, ui.ColorReset, ui.ColorBlue, *buildFolder, ui.ColorReset)
		os.Exit(1)
	}

	// Create build folder
	if err := os.MkdirAll(*buildFolder, 0755); err != nil {
		fmt.Printf("%sError:%s Cannot create build folder: %v\n", ui.ColorRed, ui.ColorReset, err)
		os.Exit(1)
	}

	fmt.Printf("Generating Makefile in folder %s%s%s ", ui.ColorCyan, *buildFolder, ui.ColorReset)

	// Build makefile
	if err := makefile.Build(*buildFolder, *srcFolder, useMusl, llvmTools, sourceFiles, libcDir, dynLinkerPath, *exeName); err != nil {
		fmt.Printf("%s%sError:%s Failed to create Makefile: %v\n", ui.ColorRed, ui.ColorReset, ui.ColorReset, err)
		os.Exit(1)
	}

	fmt.Printf("%sdone%s ...\n", ui.ColorGreen, ui.ColorReset)
	fmt.Printf("Run %smake%s in %s%s%s to build the project.\n", ui.ColorCyan, ui.ColorReset, ui.ColorCyan, *buildFolder, ui.ColorReset)
	fmt.Printf("Edit %s%s%s to modify source files and run %smake configure%s to regenerate.\n",
		ui.ColorCyan, filepath.Join(*srcFolder, *sourcesFile), ui.ColorReset, ui.ColorCyan, ui.ColorReset)
}
