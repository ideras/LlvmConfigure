package makefile

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"llvm-configure/platform"
	"llvm-configure/tools"
)

func writeSource(t *testing.T, dir, name string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte("define i32 @main() {\n  ret i32 0\n}\n"), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func buildToTemp(t *testing.T, opts BuildOptions) string {
	t.Helper()
	buildDir := t.TempDir()
	opts.BuildFolder = buildDir
	if opts.Platform == platform.PlatformDarwin && opts.SDKPath == "" {
		// Default SDK path for the test suite; the dedicated test below
		// covers the missing-SDK case explicitly.
		opts.SDKPath = "/Library/Developer/CommandLineTools/SDKs/MacOSX.sdk"
	}
	if err := Build(opts); err != nil {
		t.Fatalf("Build: %v", err)
	}
	data, err := os.ReadFile(filepath.Join(buildDir, "Makefile"))
	if err != nil {
		t.Fatalf("read generated Makefile: %v", err)
	}
	return string(data)
}

var (
	fakeTools = &tools.LLVMTools{
		LlvmAs: "/opt/homebrew/opt/llvm/bin/llvm-as",
		Llc:    "/opt/homebrew/opt/llvm/bin/llc",
		Clang:  "/usr/bin/clang",
	}
	fakeLinuxTools = &tools.LLVMTools{
		LlvmAs: "/usr/bin/llvm-as",
		Llc:    "/usr/bin/llc",
		Lld:    "/usr/bin/lld",
	}
)

func TestDarwinMakefileContent(t *testing.T) {
	src := writeSource(t, t.TempDir(), "program.ll")

	mk := buildToTemp(t, BuildOptions{
		SrcFolder:   t.TempDir(),
		SourceFiles: []string{src},
		ExeName:     "program",
		Platform:    platform.PlatformDarwin,
		Target:      platform.Target{Platform: platform.PlatformDarwin, Triple: "arm64-apple-macosx14.0.0"},
		Tools:       fakeTools,
	})

	for _, want := range []string{
		"LLVM_AS := /opt/homebrew/opt/llvm/bin/llvm-as",
		"LLC     := /opt/homebrew/opt/llvm/bin/llc",
		"CLANG   := /usr/bin/clang",
		"TARGET_TRIPLE := arm64-apple-macosx14.0.0",
		"SDK_PATH := /Library/Developer/CommandLineTools/SDKs/MacOSX.sdk",
		"\t\"$(CLANG)\" -isysroot \"$(SDK_PATH)\" -target \"$(TARGET_TRIPLE)\" $(OBJS) -o \"$(EXE)\"",
		"\t\"$(LLC)\" -mtriple=\"$(TARGET_TRIPLE)\" $< -filetype=obj -o $@",
	} {
		if !strings.Contains(mk, want) {
			t.Errorf("Darwin Makefile missing %q:\n%s", want, mk)
		}
	}
}

func TestDarwinMakefileHasNoLinuxLinkerTokens(t *testing.T) {
	src := writeSource(t, t.TempDir(), "program.ll")

	mk := buildToTemp(t, BuildOptions{
		SrcFolder:   t.TempDir(),
		SourceFiles: []string{src},
		Platform:    platform.PlatformDarwin,
		Target:      platform.Target{Platform: platform.PlatformDarwin, Triple: "arm64-apple-macosx14.0.0"},
		Tools:       fakeTools,
	})

	for _, forbidden := range []string{
		"LLD", "-flavor gnu", "-dynamic-linker", "LIBC_DIR",
		"crt1.o", "crti.o", "crtn.o", "-lc", "-lSystem", "DYNAMIC_LINKER",
	} {
		if strings.Contains(mk, forbidden) {
			t.Errorf("Darwin Makefile must not contain %q:\n%s", forbidden, mk)
		}
	}
}

func TestDarwinMakefileDeploymentTarget(t *testing.T) {
	src := writeSource(t, t.TempDir(), "program.ll")

	mk := buildToTemp(t, BuildOptions{
		SrcFolder:   t.TempDir(),
		SourceFiles: []string{src},
		Platform:    platform.PlatformDarwin,
		Target: platform.Target{
			Platform:         platform.PlatformDarwin,
			Triple:           "arm64-apple-macosx14.0.0",
			DeploymentTarget: "13.0",
		},
		Tools: fakeTools,
	})

	if !strings.Contains(mk, "TARGET_TRIPLE := arm64-apple-macosx13.0") {
		t.Errorf("deployment target must be normalized into the codegen triple:\n%s", mk)
	}
	if !strings.Contains(mk, "-mmacosx-version-min=13.0") {
		t.Errorf("validated deployment target must be passed to clang:\n%s", mk)
	}
}

func TestDarwinMakefileNoDeploymentTargetOmitsFlag(t *testing.T) {
	src := writeSource(t, t.TempDir(), "program.ll")

	mk := buildToTemp(t, BuildOptions{
		SrcFolder:   t.TempDir(),
		SourceFiles: []string{src},
		Platform:    platform.PlatformDarwin,
		Target:      platform.Target{Platform: platform.PlatformDarwin, Triple: "arm64-apple-macosx14.0.0"},
		Tools:       fakeTools,
	})

	if strings.Contains(mk, "-mmacosx-version-min") {
		t.Errorf("macosx-version-min must be emitted only when configured:\n%s", mk)
	}
}

func TestLinuxMakefileRetainsCurrentBehavior(t *testing.T) {
	src := writeSource(t, t.TempDir(), "program.ll")

	mk := buildToTemp(t, BuildOptions{
		SrcFolder:   t.TempDir(),
		SourceFiles: []string{src},
		Platform:    platform.PlatformLinux,
		Target:      platform.Target{Platform: platform.PlatformLinux},
		Tools:       fakeLinuxTools,
		LibC: &LinuxLibCOptions{
			UseMusl:       false,
			Dir:           "/usr/lib/x86_64-linux-gnu",
			DynLinkerPath: "/lib64/ld-linux-x86-64.so.2",
		},
	})

	for _, want := range []string{
		"LLD     := /usr/bin/lld",
		"$(LLD) -flavor gnu",
		"-dynamic-linker $(DYNAMIC_LINKER)",
		"DYNAMIC_LINKER := /lib64/ld-linux-x86-64.so.2",
		"LIBC_DIR := /usr/lib/x86_64-linux-gnu",
		"$(LIBC_DIR)/crt1.o",
		"$(LIBC_DIR)/crti.o",
		"$(LIBC_DIR)/crtn.o",
		"$(OBJS) -lc",
		"$(LLC) $< -filetype=obj -o $@",
	} {
		if !strings.Contains(mk, want) {
			t.Errorf("Linux Makefile missing %q:\n%s", want, mk)
		}
	}

	if strings.Contains(mk, "CLANG") || strings.Contains(mk, "TARGET_TRIPLE") {
		t.Errorf("Linux Makefile must not contain Darwin-specific tokens:\n%s", mk)
	}
}

func TestLinuxMakefileMuslLdFlags(t *testing.T) {
	src := writeSource(t, t.TempDir(), "program.ll")

	mk := buildToTemp(t, BuildOptions{
		SrcFolder:   t.TempDir(),
		SourceFiles: []string{src},
		Platform:    platform.PlatformLinux,
		Target:      platform.Target{Platform: platform.PlatformLinux},
		Tools:       fakeLinuxTools,
		LibC:        &LinuxLibCOptions{UseMusl: true, Dir: "/usr/lib/musl"},
	})

	if !strings.Contains(mk, "-static -nostdlib") {
		t.Errorf("musl linking must remain static:\n%s", mk)
	}
}

func TestDarwinBuildRequiresSDKPath(t *testing.T) {
	src := writeSource(t, t.TempDir(), "program.ll")

	opts := BuildOptions{
		SrcFolder:   t.TempDir(),
		SourceFiles: []string{src},
		Platform:    platform.PlatformDarwin,
		Target:      platform.Target{Platform: platform.PlatformDarwin, Triple: "arm64-apple-macosx14.0.0"},
		Tools:       fakeTools,
		SDKPath:     "", // deliberately missing
	}
	buildDir := t.TempDir()
	opts.BuildFolder = buildDir

	if err := Build(opts); err == nil {
		t.Fatal("Darwin builds must require a resolved macOS SDK path")
	}
}

func TestDarwinBuildRejectsLinuxLibCOptions(t *testing.T) {
	src := writeSource(t, t.TempDir(), "program.ll")

	opts := BuildOptions{
		SrcFolder:   t.TempDir(),
		SourceFiles: []string{src},
		Platform:    platform.PlatformDarwin,
		Target:      platform.Target{Platform: platform.PlatformDarwin, Triple: "arm64-apple-macosx14.0.0"},
		Tools:       fakeTools,
		LibC:        &LinuxLibCOptions{Dir: "/usr/lib/x86_64-linux-gnu"},
	}
	buildDir := t.TempDir()
	opts.BuildFolder = buildDir

	if err := Build(opts); err == nil {
		t.Fatal("Darwin builds must reject Linux libc options")
	}
}

func TestLinuxBuildRequiresLibCOptions(t *testing.T) {
	src := writeSource(t, t.TempDir(), "program.ll")

	opts := BuildOptions{
		SrcFolder:   t.TempDir(),
		SourceFiles: []string{src},
		Platform:    platform.PlatformLinux,
		Target:      platform.Target{Platform: platform.PlatformLinux},
		Tools:       fakeLinuxTools,
	}
	buildDir := t.TempDir()
	opts.BuildFolder = buildDir

	if err := Build(opts); err == nil {
		t.Fatal("Linux builds require libc linking options")
	}
}

func TestRecipesUseTabs(t *testing.T) {
	src := writeSource(t, t.TempDir(), "program.ll")

	mk := buildToTemp(t, BuildOptions{
		SrcFolder:   t.TempDir(),
		SourceFiles: []string{src},
		Platform:    platform.PlatformDarwin,
		Target:      platform.Target{Platform: platform.PlatformDarwin, Triple: "arm64-apple-macosx14.0.0"},
		Tools:       fakeTools,
	})

	for _, line := range strings.Split(mk, "\n") {
		trimmed := strings.TrimLeft(line, " ")
		if strings.HasPrefix(trimmed, `"$(LLC)"`) || strings.HasPrefix(trimmed, `"$(CLANG)"`) || strings.HasPrefix(trimmed, "$(LLVM_AS)") {
			if !strings.HasPrefix(line, "\t") {
				t.Errorf("recipe line must start with a tab: %q", line)
			}
		}
	}
}
