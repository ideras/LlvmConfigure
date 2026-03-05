# LLVM tools and filenames
LLVM_AS := {{.LlvmAs}}
LLC     := {{.Llc}}
LLD     := {{.Lld}}

# Source files
SRCS     := {{.SrcFiles}}
BITCODES := {{.Bitcodes}}
OBJS     := {{.Objs}}
EXE      := {{.ExeName}}

# System dynamic linker
DYNAMIC_LINKER := {{.DynLinkerPath}}
LIBC_DIR := {{.LibcDir}}

.PHONY: all clean rebuild configure

all: $(EXE)

# Pattern rule: Generate object files from bitcode
%.o: %.bc
	$(LLC) $< -filetype=obj -o $@

# Link all object files into an executable
$(EXE): $(OBJS)
	$(LLD) -flavor gnu \
		-o $(EXE) \
		{{.LdFlags}} \
		-L $(LIBC_DIR) \
		$(LIBC_DIR)/crt1.o \
		$(LIBC_DIR)/crti.o \
		$(OBJS) -lc \
		$(LIBC_DIR)/crtn.o

clean:
	rm -f $(BITCODES) $(OBJS) $(EXE)

rebuild: clean all

# Reconfigure using LLVMSources.txt
configure:
	@echo "Re-running configuration from LLVMSources.txt..."
	@{{.ExePath}} -B {{.BuildFolder}} -S {{.SrcFolder}}

# Individual bitcode targets
{{ range .BitcodeTargets }}
{{.BaseName}}.bc: {{.SrcFile}}
	$(LLVM_AS) $< -o $@
{{ end }}