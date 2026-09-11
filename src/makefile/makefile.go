package makefile

import (
	_ "embed"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"llvm-configure/platform"
	"llvm-configure/tools"
)

type BitcodeTarget struct {
	BaseName string
	SrcFile  string
}

// LinuxLibCOptions carries the Linux libc linking strategy.
// It must be nil on Darwin, where clang and the macOS SDK select startup
// behavior and libSystem.
type LinuxLibCOptions struct {
	UseMusl       bool
	Dir           string
	DynLinkerPath string
}

// BuildOptions is the structured input of makefile.Build.
type BuildOptions struct {
	BuildFolder string
	SrcFolder   string
	SourceFiles []string
	ExeName     string
	Platform    platform.PlatformKind
	Target      platform.Target
	Tools       *tools.LLVMTools
	// SDKPath is the active macOS SDK sysroot, resolved at build time
	// via xcrun. Required on Darwin; empty on Linux. It is passed to the
	// link command as -isysroot because a clang binary invoked directly
	// (as returned by xcrun --find) does not set a default sysroot.
	SDKPath string
	LibC    *LinuxLibCOptions
}

type MakefileData struct {
	LlvmAs         string
	Llc            string
	Lld            string
	SrcFiles       string
	Bitcodes       string
	Objs           string
	ExeName        string
	DynLinkerPath  string
	LibcDir        string
	LdFlags        string
	ExePath        string
	BuildFolder    string
	SrcFolder      string
	BitcodeTargets []BitcodeTarget
}

// DarwinMakefileData is the template data for the Darwin/Mach-O Makefile.
type DarwinMakefileData struct {
	LlvmAs           string
	Llc              string
	Clang            string
	TargetTriple     string
	MacosxVersionMin string
	SDKPath          string
	SrcFiles         string
	Bitcodes         string
	Objs             string
	ExeName          string
	ExePath          string
	BuildFolder      string
	SrcFolder        string
	BitcodeTargets   []BitcodeTarget
}

//go:embed makefile.linux.templ.mk
var linuxMakefileTemplate string

//go:embed makefile.darwin.templ.mk
var darwinMakefileTemplate string

// Build generates the Makefile in the build folder.
//
// The Linux path preserves the historical ELF/GNU-libc behavior byte for
// byte (except shell-quoting of the configure command); the Darwin path
// emits Mach-O code generation through llc -mtriple and links through
// clang. LibC must be nil on Darwin.
func Build(opts BuildOptions) error {
	switch opts.Platform {
	case platform.PlatformLinux:
		return buildLinux(opts)
	case platform.PlatformDarwin:
		return buildDarwin(opts)
	default:
		return fmt.Errorf("unsupported platform %q", string(opts.Platform))
	}
}

// buildLinux generates the historical Linux Makefile.
func buildLinux(opts BuildOptions) error {
	if opts.LibC == nil {
		return fmt.Errorf("linux builds require libc linking options")
	}

	var ldFlags string
	if opts.LibC.UseMusl {
		ldFlags = "-static -nostdlib"
	} else {
		ldFlags = "-dynamic-linker $(DYNAMIC_LINKER)"
	}

	common, err := prepareCommonData(opts)
	if err != nil {
		return err
	}

	data := MakefileData{
		LlvmAs:         opts.Tools.LlvmAs,
		Llc:            opts.Tools.Llc,
		Lld:            opts.Tools.Lld,
		SrcFiles:       common.srcFiles,
		Bitcodes:       common.bitcodes,
		Objs:           common.objs,
		ExeName:        common.exeName,
		DynLinkerPath:  opts.LibC.DynLinkerPath,
		LibcDir:        opts.LibC.Dir,
		LdFlags:        ldFlags,
		ExePath:        common.exePath,
		BuildFolder:    opts.BuildFolder,
		SrcFolder:      opts.SrcFolder,
		BitcodeTargets: common.targets,
	}

	return executeTemplate(linuxMakefileTemplate, data, opts.BuildFolder)
}

// buildDarwin generates the Darwin/Mach-O Makefile.
func buildDarwin(opts BuildOptions) error {
	if opts.LibC != nil {
		return fmt.Errorf("darwin builds must not carry Linux libc options")
	}
	if opts.SDKPath == "" {
		return fmt.Errorf("darwin builds require a resolved macOS SDK path")
	}

	common, err := prepareCommonData(opts)
	if err != nil {
		return err
	}

	// Centralized triple/deployment-target normalization so llc code
	// generation and clang link metadata always agree.
	codegenTriple, versionMin, err := platform.CodegenTriple(opts.Target.Triple, opts.Target.DeploymentTarget)
	if err != nil {
		return err
	}

	data := DarwinMakefileData{
		LlvmAs:           opts.Tools.LlvmAs,
		Llc:              opts.Tools.Llc,
		Clang:            opts.Tools.Clang,
		TargetTriple:     codegenTriple,
		MacosxVersionMin: versionMin,
		SDKPath:          opts.SDKPath,
		SrcFiles:         common.srcFiles,
		Bitcodes:         common.bitcodes,
		Objs:             common.objs,
		ExeName:          common.exeName,
		ExePath:          common.exePath,
		BuildFolder:      opts.BuildFolder,
		SrcFolder:        opts.SrcFolder,
		BitcodeTargets:   common.targets,
	}

	return executeTemplate(darwinMakefileTemplate, data, opts.BuildFolder)
}

type commonMakefileData struct {
	srcFiles string
	bitcodes string
	objs     string
	exeName  string
	exePath  string
	targets  []BitcodeTarget
}

// prepareCommonData computes source lists, bitcode/object names, and the
// executable name shared by both platform templates.
func prepareCommonData(opts BuildOptions) (*commonMakefileData, error) {
	if len(opts.SourceFiles) == 0 {
		return nil, fmt.Errorf("no source files provided")
	}

	// Generate relative paths from build folder
	var relSrcFiles []string

	buildFolderAbs, err := filepath.Abs(opts.BuildFolder)
	if err != nil {
		buildFolderAbs = opts.BuildFolder
	}

	for _, asmFile := range opts.SourceFiles {
		relPath, err := filepath.Rel(buildFolderAbs, asmFile)
		if err != nil {
			relPath = asmFile
		}
		relSrcFiles = append(relSrcFiles, relPath)
	}

	// Generate file lists
	srcFiles := strings.Join(relSrcFiles, " ")

	bcFiles := make([]string, len(opts.SourceFiles))
	objFiles := make([]string, len(opts.SourceFiles))
	targets := make([]BitcodeTarget, len(opts.SourceFiles))

	for i, asmFile := range opts.SourceFiles {
		base := strings.TrimSuffix(filepath.Base(asmFile), ".ll")
		bcFiles[i] = base + ".bc"
		objFiles[i] = base + ".o"

		targets[i] = BitcodeTarget{
			BaseName: base,
			SrcFile:  relSrcFiles[i],
		}
	}

	exeName := opts.ExeName
	if exeName == "" {
		exeName = strings.TrimSuffix(filepath.Base(opts.SourceFiles[0]), ".ll")
	}

	// Get absolute path to this executable
	exePath, _ := os.Executable()

	return &commonMakefileData{
		srcFiles: srcFiles,
		bitcodes: strings.Join(bcFiles, " "),
		objs:     strings.Join(objFiles, " "),
		exeName:  exeName,
		exePath:  exePath,
		targets:  targets,
	}, nil
}

// executeTemplate renders a platform template into the build folder.
func executeTemplate(templateText string, data interface{}, buildFolder string) error {
	makefilePath := filepath.Join(buildFolder, "Makefile")
	file, err := os.Create(makefilePath)
	if err != nil {
		return err
	}
	defer file.Close()

	tmpl := template.Must(template.New("makefile").Parse(templateText))

	return tmpl.Execute(file, data)
}
