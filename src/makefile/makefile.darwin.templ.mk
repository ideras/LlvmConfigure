# LLVM tools and filenames (Darwin / Mach-O)
LLVM_AS := {{.LlvmAs}}
LLC     := {{.Llc}}
CLANG   := {{.Clang}}

TARGET_TRIPLE := {{.TargetTriple}}

# Active macOS SDK, resolved at make time through xcrun. It is passed to
# clang as -isysroot because a clang binary invoked directly (as returned
# by xcrun --find) does not set a default sysroot.
SDK_PATH := {{.SDKPath}}

# Source files
SRCS     := {{.SrcFiles}}
BITCODES := {{.Bitcodes}}
OBJS     := {{.Objs}}
EXE      := {{.ExeName}}

.PHONY: all clean rebuild configure

all: $(EXE)

# Pattern rule: generate Mach-O object files from bitcode
%.o: %.bc
	"$(LLC)" -mtriple="$(TARGET_TRIPLE)" $< -filetype=obj -o $@

# Link all object files into a Mach-O executable through clang.
# Clang selects the SDK startup behavior and libSystem; no raw GNU
# linker driver, CRT objects, dynamic linker, or explicit libc flags
# are involved.
$(EXE): $(OBJS)
	"$(CLANG)" -isysroot "$(SDK_PATH)" -target "$(TARGET_TRIPLE)" {{ if .MacosxVersionMin }}-mmacosx-version-min={{.MacosxVersionMin}} {{ end }}$(OBJS) -o "$(EXE)"

clean:
	rm -f $(BITCODES) $(OBJS) $(EXE)

rebuild: clean all

configure:
	@echo "Re-running configuration from LLVMSources.txt..."
	@'{{.ExePath}}' -B '{{.BuildFolder}}' -S '{{.SrcFolder}}'

# Individual bitcode targets
{{ range .BitcodeTargets }}
{{.BaseName}}.bc: {{.SrcFile}}
	$(LLVM_AS) $< -o $@
{{ end }}