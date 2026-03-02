package makefile

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"llvm-configure/tools"
)

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

	// Get absolute path to this executable
	exePath, _ := os.Executable()

	fmt.Fprintf(file, "# LLVM tools and filenames\n")
	fmt.Fprintf(file, "LLVM_AS := %s\n", llvmTools.LlvmAs)
	fmt.Fprintf(file, "LLC     := %s\n", llvmTools.Llc)
	fmt.Fprintf(file, "LLD     := %s\n", llvmTools.Lld)
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
