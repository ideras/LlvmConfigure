package main

import (
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"llvm-configure/config"
	"llvm-configure/makefile"
	"llvm-configure/platform"
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
	withMusl := flag.Bool("with-musl", false, "Use MUSL C library instead of GNU libc (Linux only)")
	exeName := flag.String("exe-name", "", "Executable name (default: first source file basename)")
	sourcesFile := flag.String("sources-file", "LLVMSources.txt", "Source list file name")
	checkLLVM := flag.Bool("check-llvm", false, "Check whether required LLVM tools are installed")
	checkLibC := flag.Bool("check-libc", false, "Check whether GNU libc is installed")
	checkMusl := flag.Bool("check-musl", false, "Check whether musl libc is installed")
	scanLLVM := flag.Bool("scan-llvm", false, "Scan the system for LLVM tools (and, on macOS, the target triple) and update the config file")
	scanLibC := flag.Bool("scan-libc", false, "Scan the system for GNU libc and update the config file (Linux only)")

	flag.Parse()

	// Select the native build platform once. Unsupported operating
	// systems fail here instead of accidentally entering the Linux path.
	plat, err := platform.Detect()
	if err != nil {
		fmt.Printf("%sError:%s %v\n", ui.ColorRed, ui.ColorReset, err)
		os.Exit(1)
	}

	// --with-musl is rejected on Darwin before anything else, in
	// particular before any build directory is created.
	if plat == platform.PlatformDarwin && *withMusl {
		fmt.Printf("%sError:%s --with-musl is not supported on macOS; macOS executables are linked against libSystem by clang.\n", ui.ColorRed, ui.ColorReset)
		os.Exit(1)
	}

	if *checkLLVM || *checkLibC || *checkMusl || *scanLLVM || *scanLibC {
		cfg, err := config.LoadConfigFile()
		if err != nil {
			fmt.Printf("%sWarning:%s Could not load config file: %v\n", ui.ColorYellow, ui.ColorReset, err)
			cfg = &config.Config{}
		}

		exitCode := 0
		if *scanLLVM {
			exitCode = max(exitCode, handleScanLLVM(plat, cfg))
		}
		if *scanLibC {
			exitCode = max(exitCode, handleScanLibC(plat, cfg))
		}
		if *checkLLVM {
			exitCode = max(exitCode, handleCheckLLVM(plat, cfg))
		}
		if *checkLibC {
			exitCode = max(exitCode, handleCheckLibC(plat))
		}
		if *checkMusl {
			exitCode = max(exitCode, handleCheckMusl(plat))
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

	if plat == platform.PlatformDarwin {
		runDarwinBuild(cfg, *buildFolder, *srcFolder, *exeName, *sourcesFile, sourceFiles)
	} else {
		runLinuxBuild(cfg, *buildFolder, *srcFolder, *exeName, *sourcesFile, sourceFiles, *withMusl)
	}
}

// handleScanLLVM discovers the platform's LLVM toolchain and, on Darwin,
// the native target triple, then persists them together.
func handleScanLLVM(plat platform.PlatformKind, cfg *config.Config) int {
	runner := platform.DefaultRunner

	if plat == platform.PlatformDarwin {
		llvmTools, err := tools.DiscoverDarwinLLVMTools(cfg, runner)
		if err != nil {
			fmt.Printf("%sLLVM tool scan failed:%s %v\n", ui.ColorRed, ui.ColorReset, err)
			return 1
		}

		if err := platform.ValidateMacSDK(runner); err != nil {
			fmt.Printf("%sLLVM tool scan failed:%s %v\n", ui.ColorRed, ui.ColorReset, err)
			return 1
		}

		triple, err := platform.DetectTargetTriple(llvmTools.Clang, runner)
		if err != nil {
			fmt.Printf("%sLLVM tool scan failed:%s %v\n", ui.ColorRed, ui.ColorReset, err)
			return 1
		}

		codegenTriple, _, err := platform.CodegenTriple(triple, cfg.Target.DeploymentTarget)
		if err != nil {
			fmt.Printf("%sLLVM tool scan failed:%s %v\n", ui.ColorRed, ui.ColorReset, err)
			return 1
		}

		if err := platform.ValidateLLCTriple(llvmTools.Llc, codegenTriple, runner); err != nil {
			fmt.Printf("%sLLVM tool scan failed:%s %v\n", ui.ColorRed, ui.ColorReset, err)
			return 1
		}

		cfg.LLVM = config.LLVMConfig{
			LlvmAs: llvmTools.LlvmAs,
			Llc:    llvmTools.Llc,
			Lld:    "",
			Clang:  llvmTools.Clang,
		}
		cfg.Target = config.TargetConfig{
			Triple:           triple,
			DeploymentTarget: cfg.Target.DeploymentTarget,
			DetectedBy:       llvmTools.Clang,
		}

		if err := config.SaveConfigFile(cfg); err != nil {
			fmt.Printf("%sFailed to update LLVM configuration:%s %v\n", ui.ColorRed, ui.ColorReset, err)
			return 1
		}

		fmt.Printf("%sUpdated LLVM tool paths and target triple in the configuration file.%s\n", ui.ColorGreen, ui.ColorReset)
		fmt.Printf("Target triple: %s%s%s (detected by %s%s%s)\n", ui.ColorCyan, triple, ui.ColorReset, ui.ColorCyan, llvmTools.Clang, ui.ColorReset)
		return 0
	}

	llvmTools, err := tools.ScanLLVMTools(plat)
	if err != nil {
		fmt.Printf("%sLLVM tool scan failed:%s %v\n", ui.ColorRed, ui.ColorReset, err)
		return 1
	}

	cfg.LLVM = config.LLVMConfig{
		LlvmAs: llvmTools.LlvmAs,
		Llc:    llvmTools.Llc,
		Lld:    llvmTools.Lld,
	}
	if err := config.SaveConfigFile(cfg); err != nil {
		fmt.Printf("%sFailed to update LLVM configuration:%s %v\n", ui.ColorRed, ui.ColorReset, err)
		return 1
	}

	fmt.Printf("%sUpdated LLVM tool paths in the configuration file.%s\n", ui.ColorGreen, ui.ColorReset)
	return 0
}

// handleScanLibC scans for GNU libc. On Darwin libc scanning is not
// applicable and must not modify the configuration.
func handleScanLibC(plat platform.PlatformKind, cfg *config.Config) int {
	if plat == platform.PlatformDarwin {
		fmt.Printf("GNU libc scan is not applicable on macOS; macOS executables are linked against libSystem by clang.\n")
		return 0
	}

	libcPaths, err := system.FindLibC()
	libcPath, found := libcPaths["gnu"]
	if err != nil {
		fmt.Printf("%sGNU libc scan failed:%s %v\n", ui.ColorRed, ui.ColorReset, err)
		return 1
	} else if !found {
		fmt.Printf("%sGNU libc scan failed:%s GNU libc not found\n", ui.ColorRed, ui.ColorReset)
		return 1
	} else if err := system.CheckObjectFiles(libcPath); err != nil {
		fmt.Printf("%sGNU libc scan failed:%s %v\n", ui.ColorRed, ui.ColorReset, err)
		return 1
	} else {
		dynLinkerPath, err := system.GetDynamicLinkerPath()
		if err != nil {
			fmt.Printf("%sGNU libc scan failed:%s cannot find dynamic linker: %v\n", ui.ColorRed, ui.ColorReset, err)
			return 1
		} else {
			cfg.LibC = config.LibCConfig{
				Path:          libcPath,
				DynLinkerPath: dynLinkerPath,
			}
			if err := config.SaveConfigFile(cfg); err != nil {
				fmt.Printf("%sFailed to update GNU libc configuration:%s %v\n", ui.ColorRed, ui.ColorReset, err)
				return 1
			} else {
				fmt.Printf("%sUpdated GNU libc and dynamic linker paths in the configuration file.%s\n", ui.ColorGreen, ui.ColorReset)
			}
		}
	}

	return 0
}

// handleCheckLLVM validates the platform's required toolchain. On Darwin
// it additionally validates the clang linker driver, the macOS SDK, and
// the stored/detected target triple.
func handleCheckLLVM(plat platform.PlatformKind, cfg *config.Config) int {
	runner := platform.DefaultRunner

	if plat == platform.PlatformDarwin {
		llvmTools, err := tools.DiscoverDarwinLLVMTools(cfg, runner)
		if err != nil {
			fmt.Printf("%sLLVM tools not found:%s %v\n", ui.ColorRed, ui.ColorReset, err)
			return 1
		}
		fmt.Printf("%sllvm-as: %s%s\n", ui.ColorGreen, llvmTools.LlvmAs, ui.ColorReset)
		fmt.Printf("%sllc: %s%s\n", ui.ColorGreen, llvmTools.Llc, ui.ColorReset)
		fmt.Printf("%sclang (linker driver): %s%s\n", ui.ColorGreen, llvmTools.Clang, ui.ColorReset)

		if err := platform.ValidateMacSDK(runner); err != nil {
			fmt.Printf("%sLLVM tool check failed:%s %v\n", ui.ColorRed, ui.ColorReset, err)
			return 1
		}
		fmt.Printf("%smacOS SDK is available.%s\n", ui.ColorGreen, ui.ColorReset)

		triple, err := platform.DetectTargetTriple(llvmTools.Clang, runner)
		if err != nil {
			fmt.Printf("%sLLVM tool check failed:%s %v\n", ui.ColorRed, ui.ColorReset, err)
			return 1
		}

		if cfg.Target.Triple != "" {
			if err := platform.ValidateStoredTarget(cfg.Target.Triple, triple); err != nil {
				fmt.Printf("%sLLVM tool check failed:%s %v\n", ui.ColorRed, ui.ColorReset, err)
				return 1
			}
			fmt.Printf("%sStored target triple %s%s%s matches the detected target.%s\n", ui.ColorGreen, ui.ColorCyan, cfg.Target.Triple, ui.ColorGreen, ui.ColorReset)
		} else {
			fmt.Printf("Detected target triple: %s%s%s (not stored; run --scan-llvm to persist it)\n", ui.ColorCyan, triple, ui.ColorReset)
		}

		codegenTriple, _, err := platform.CodegenTriple(triple, cfg.Target.DeploymentTarget)
		if err != nil {
			fmt.Printf("%sLLVM tool check failed:%s %v\n", ui.ColorRed, ui.ColorReset, err)
			return 1
		}
		if err := platform.ValidateLLCTriple(llvmTools.Llc, codegenTriple, runner); err != nil {
			fmt.Printf("%sLLVM tool check failed:%s %v\n", ui.ColorRed, ui.ColorReset, err)
			return 1
		}
		fmt.Printf("%sLLVM tools and macOS target are valid.%s\n", ui.ColorGreen, ui.ColorReset)
		return 0
	}

	if _, err := tools.CheckLLVMTools(cfg, plat); err != nil {
		fmt.Printf("%sLLVM tools not found:%s %v\n", ui.ColorRed, ui.ColorReset, err)
		return 1
	}

	fmt.Printf("%sLLVM tools are installed.%s\n", ui.ColorGreen, ui.ColorReset)
	return 0
}

// handleCheckLibC reports GNU libc availability. Not applicable on Darwin.
func handleCheckLibC(plat platform.PlatformKind) int {
	if plat == platform.PlatformDarwin {
		fmt.Printf("GNU libc check is not applicable on macOS; macOS executables are linked against libSystem by clang.\n")
		return 0
	}

	libcPaths, err := system.FindLibC()
	libcPath, found := libcPaths["gnu"]
	if err != nil || !found {
		fmt.Printf("%sGNU libc not found.%s\n", ui.ColorRed, ui.ColorReset)
		return 1
	} else if err := system.CheckObjectFiles(libcPath); err != nil {
		fmt.Printf("%sGNU libc is incomplete:%s %v\n", ui.ColorRed, ui.ColorReset, err)
		return 1
	} else {
		fmt.Printf("%sGNU libc is installed at %s%s%s\n", ui.ColorGreen, ui.ColorCyan, libcPath, ui.ColorReset)
	}

	return 0
}

// handleCheckMusl reports musl availability. Not applicable on Darwin.
func handleCheckMusl(plat platform.PlatformKind) int {
	if plat == platform.PlatformDarwin {
		fmt.Printf("musl libc check is not applicable on macOS; macOS executables are linked against libSystem by clang.\n")
		return 0
	}

	libcPaths, err := system.FindLibC()
	muslPath, found := libcPaths["musl"]
	if err != nil || !found {
		fmt.Printf("%smusl libc not found.%s\n", ui.ColorRed, ui.ColorReset)
		return 1
	} else {
		fmt.Printf("%smusl libc is installed at %s%s%s\n", ui.ColorGreen, ui.ColorCyan, muslPath, ui.ColorReset)
	}

	return 0
}

// runDarwinBuild configures a native macOS build. It never invokes Linux
// libc, CRT object, or ELF interpreter discovery.
func runDarwinBuild(cfg *config.Config, buildFolder, srcFolder, exeName, sourcesFile string, sourceFiles []string) {
	runner := platform.DefaultRunner

	fmt.Printf("Selected platform: %smacOS (Darwin)%s\n", ui.ColorCyan, ui.ColorReset)

	// Discover llvm-as, llc, and clang.
	llvmTools, err := tools.DiscoverDarwinLLVMTools(cfg, runner)
	if err != nil {
		fmt.Printf("%sError:%s %v\n", ui.ColorRed, ui.ColorReset, err)
		os.Exit(1)
	}
	fmt.Printf("Using llvm-as: %s%s%s\n", ui.ColorCyan, llvmTools.LlvmAs, ui.ColorReset)
	fmt.Printf("Using llc: %s%s%s\n", ui.ColorCyan, llvmTools.Llc, ui.ColorReset)
	fmt.Printf("Using clang (linker driver): %s%s%s\n", ui.ColorCyan, llvmTools.Clang, ui.ColorReset)

	// Validate the macOS SDK and resolve its path for the link command.
	sdkPath, err := platform.MacSDKPath(runner)
	if err != nil {
		fmt.Printf("%sError:%s %v\n", ui.ColorRed, ui.ColorReset, err)
		os.Exit(1)
	}
	fmt.Printf("%smacOS SDK: %s%s%s\n", ui.ColorGreen, ui.ColorCyan, sdkPath, ui.ColorReset)

	// Resolve the target triple. The selected clang toolchain is the
	// authority for the effective native target.
	detected, err := platform.DetectTargetTriple(llvmTools.Clang, runner)
	if err != nil {
		fmt.Printf("%sError:%s %v\n", ui.ColorRed, ui.ColorReset, err)
		os.Exit(1)
	}

	triple := detected
	if cfg.Target.Triple != "" {
		if err := platform.ValidateStoredTarget(cfg.Target.Triple, detected); err != nil {
			fmt.Printf("%sError:%s %v\n", ui.ColorRed, ui.ColorReset, err)
			os.Exit(1)
		}
		triple = cfg.Target.Triple
		fmt.Printf("Using stored target triple: %s%s%s (detected by %s%s%s)\n",
			ui.ColorCyan, triple, ui.ColorReset, ui.ColorCyan, cfg.Target.DetectedBy, ui.ColorReset)
	} else {
		fmt.Printf("Detected target triple: %s%s%s (not persisted in the configuration; run --scan-llvm to store it)\n",
			ui.ColorCyan, triple, ui.ColorReset)
	}

	// Validate deployment target policy, if configured.
	if cfg.Target.DeploymentTarget != "" {
		if err := platform.ValidateMacosxVersion(cfg.Target.DeploymentTarget); err != nil {
			fmt.Printf("%sError:%s %v\n", ui.ColorRed, ui.ColorReset, err)
			os.Exit(1)
		}
		fmt.Printf("Using macOS deployment target: %s%s%s\n", ui.ColorCyan, cfg.Target.DeploymentTarget, ui.ColorReset)
	}

	// Centralized normalization so llc and clang metadata always agree.
	codegenTriple, versionMin, err := platform.CodegenTriple(triple, cfg.Target.DeploymentTarget)
	if err != nil {
		fmt.Printf("%sError:%s %v\n", ui.ColorRed, ui.ColorReset, err)
		os.Exit(1)
	}
	if versionMin != "" {
		fmt.Printf("Code-generation triple: %s%s%s\n", ui.ColorCyan, codegenTriple, ui.ColorReset)
	}

	// Verify the selected llc accepts the triple.
	if err := platform.ValidateLLCTriple(llvmTools.Llc, codegenTriple, runner); err != nil {
		fmt.Printf("%sError:%s %v\n", ui.ColorRed, ui.ColorReset, err)
		os.Exit(1)
	}

	// Warn on explicit, incompatible target triples declared inside the
	// input .ll files; llc -mtriple overrides them.
	for _, warning := range platform.FindConflictingSourceTriples(sourceFiles, codegenTriple) {
		fmt.Printf("%sWarning:%s %s\n", ui.ColorYellow, ui.ColorReset, warning)
	}

	// Check if build folder exists
	if _, err := os.Stat(buildFolder); !os.IsNotExist(err) {
		fmt.Printf("%sError:%s Target directory %s%s%s already exists\n", ui.ColorRed, ui.ColorReset, ui.ColorBlue, buildFolder, ui.ColorReset)
		os.Exit(1)
	}

	// Create build folder
	if err := os.MkdirAll(buildFolder, 0755); err != nil {
		fmt.Printf("%sError:%s Cannot create build folder: %v\n", ui.ColorRed, ui.ColorReset, err)
		os.Exit(1)
	}

	fmt.Printf("Generating Makefile in folder %s%s%s ", ui.ColorCyan, buildFolder, ui.ColorReset)

	if err := makefile.Build(makefile.BuildOptions{
		BuildFolder: buildFolder,
		SrcFolder:   srcFolder,
		SourceFiles: sourceFiles,
		ExeName:     exeName,
		Platform:    platform.PlatformDarwin,
		Target: platform.Target{
			Platform:         platform.PlatformDarwin,
			Triple:           triple,
			DeploymentTarget: cfg.Target.DeploymentTarget,
		},
		Tools: llvmTools,
		// Resolved at build time via xcrun; never persisted in config.
		SDKPath: sdkPath,
		// LibC is nil on Darwin by construction.
	}); err != nil {
		fmt.Printf("%s%sError:%s Failed to create Makefile: %v\n", ui.ColorRed, ui.ColorReset, ui.ColorReset, err)
		os.Exit(1)
	}

	fmt.Printf("%sdone%s ...\n", ui.ColorGreen, ui.ColorReset)
	fmt.Printf("Run %smake%s in %s%s%s to build the project.\n", ui.ColorCyan, ui.ColorReset, ui.ColorCyan, buildFolder, ui.ColorReset)
	fmt.Printf("Edit %s%s%s to modify source files and run %smake configure%s to regenerate.\n",
		ui.ColorCyan, filepath.Join(srcFolder, sourcesFile), ui.ColorReset, ui.ColorCyan, ui.ColorReset)
}

// runLinuxBuild preserves the historical Linux build flow: libc, CRT
// objects, and the ELF dynamic linker are discovered and referenced by the
// generated Makefile, and linking is performed with GNU-flavor lld.
func runLinuxBuild(cfg *config.Config, buildFolder, srcFolder, exeName, sourcesFile string, sourceFiles []string, withMusl bool) {
	// Use MUSL from config if not specified on command line
	useMusl := withMusl || cfg.LibC.UseMusl

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
			dynLinker, err := system.GetDynamicLinkerPath()
			if err != nil {
				fmt.Printf("%sError:%s Cannot find linux dynamic linker\n", ui.ColorRed, ui.ColorReset)
				os.Exit(1)
			}
			dynLinkerPath = dynLinker
			fmt.Printf("Found Linux dynamic linker at %s%s%s\n", ui.ColorCyan, dynLinkerPath, ui.ColorReset)
		}
	}

	// Check object files
	if err := system.CheckObjectFiles(libcDir); err != nil {
		fmt.Printf("%sError:%s %v\n", ui.ColorRed, ui.ColorReset, err)
		os.Exit(1)
	}

	// Find LLVM tools
	llvmTools, err := tools.FindLLVMTools(cfg, platform.PlatformLinux)
	if err != nil {
		fmt.Printf("%sError:%s %v\n", ui.ColorRed, ui.ColorReset, err)
		os.Exit(1)
	}

	// Check if build folder exists
	if _, err := os.Stat(buildFolder); !os.IsNotExist(err) {
		fmt.Printf("%sError:%s Target directory %s%s%s already exists\n", ui.ColorRed, ui.ColorReset, ui.ColorBlue, buildFolder, ui.ColorReset)
		os.Exit(1)
	}

	// Create build folder
	if err := os.MkdirAll(buildFolder, 0755); err != nil {
		fmt.Printf("%sError:%s Cannot create build folder: %v\n", ui.ColorRed, ui.ColorReset, err)
		os.Exit(1)
	}

	fmt.Printf("Generating Makefile in folder %s%s%s ", ui.ColorCyan, buildFolder, ui.ColorReset)

	// Build makefile
	if err := makefile.Build(makefile.BuildOptions{
		BuildFolder: buildFolder,
		SrcFolder:   srcFolder,
		SourceFiles: sourceFiles,
		ExeName:     exeName,
		Platform:    platform.PlatformLinux,
		Target:      platform.Target{Platform: platform.PlatformLinux},
		Tools:       llvmTools,
		LibC: &makefile.LinuxLibCOptions{
			UseMusl:       useMusl,
			Dir:           libcDir,
			DynLinkerPath: dynLinkerPath,
		},
	}); err != nil {
		fmt.Printf("%s%sError:%s Failed to create Makefile: %v\n", ui.ColorRed, ui.ColorReset, ui.ColorReset, err)
		os.Exit(1)
	}

	fmt.Printf("%sdone%s ...\n", ui.ColorGreen, ui.ColorReset)
	fmt.Printf("Run %smake%s in %s%s%s to build the project.\n", ui.ColorCyan, ui.ColorReset, ui.ColorCyan, buildFolder, ui.ColorReset)
	fmt.Printf("Edit %s%s%s to modify source files and run %smake configure%s to regenerate.\n",
		ui.ColorCyan, filepath.Join(srcFolder, sourcesFile), ui.ColorReset, ui.ColorCyan, ui.ColorReset)
}
