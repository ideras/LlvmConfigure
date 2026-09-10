package makefile

import (
	_ "embed"
	"os"
	"path/filepath"
	"strings"
	"text/template"

	"llvm-configure/tools"
)

type BitcodeTarget struct {
	BaseName string
	SrcFile  string
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

//go:embed makefile.templ.mk
var makefileTemplate string

// Build generates the Makefile in the build folder
func Build(buildFolder, srcFolder string, useMusl bool, llvmTools *tools.LLVMTools, asmFiles []string, libcDir, dynLinkerPath, exeName string) error {
	var ldFlags string
	if useMusl {
		ldFlags = "-static -nostdlib"
	} else {
		ldFlags = "-dynamic-linker $(DYNAMIC_LINKER)"
	}

	// Generate relative paths from build folder
	var relSrcFiles []string

	buildFolderAbs, err := filepath.Abs(buildFolder)
	if err != nil {
		buildFolderAbs = buildFolder
	}

	for _, asmFile := range asmFiles {
		relPath, err := filepath.Rel(buildFolderAbs, asmFile)
		if err != nil {
			relPath = asmFile
		}
		relSrcFiles = append(relSrcFiles, relPath)
	}

	// Generate file lists
	srcFiles := strings.Join(relSrcFiles, " ")

	bcFiles := make([]string, len(asmFiles))
	objFiles := make([]string, len(asmFiles))
	targets := make([]BitcodeTarget, len(asmFiles))

	for i, asmFile := range asmFiles {
		base := strings.TrimSuffix(filepath.Base(asmFile), ".ll")
		bcFiles[i] = base + ".bc"
		objFiles[i] = base + ".o"

		targets[i] = BitcodeTarget{
			BaseName: base,
			SrcFile:  relSrcFiles[i],
		}
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

	// Get absolute path to this executable
	exePath, _ := os.Executable()

	tmpl := template.Must(template.New("makefile").Parse(makefileTemplate))

	data := MakefileData{
		LlvmAs:         llvmTools.LlvmAs,
		Llc:            llvmTools.Llc,
		Lld:            llvmTools.Lld,
		SrcFiles:       srcFiles,
		Bitcodes:       strings.Join(bcFiles, " "),
		Objs:           strings.Join(objFiles, " "),
		ExeName:        exeName,
		DynLinkerPath:  dynLinkerPath,
		LibcDir:        libcDir,
		LdFlags:        ldFlags,
		ExePath:        exePath,
		BuildFolder:    buildFolder,
		SrcFolder:      srcFolder,
		BitcodeTargets: targets,
	}

	return tmpl.Execute(file, data)
}
