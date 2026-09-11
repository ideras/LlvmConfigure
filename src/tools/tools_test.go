package tools

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"llvm-configure/config"
	"llvm-configure/platform"
)

// fakeDarwinRunner simulates command lookup and execution with an
// on-disk bin directory so isExecutableFile performs real validation.
type fakeDarwinRunner struct {
	binDir   string // directory holding the fake executables
	pathBin  string // directory reachable through fake PATH
	lookPath map[string]string
	results  map[string]runResult
}

type runResult struct {
	stdout string
	stderr string
	err    error
}

func (f *fakeDarwinRunner) LookPath(file string) (string, error) {
	if path, ok := f.lookPath[file]; ok {
		return path, nil
	}
	return "", errNotFound{}
}

type errNotFound struct{}

func (errNotFound) Error() string { return "executable file not found in $PATH" }

func (f *fakeDarwinRunner) Run(spec platform.CommandSpec) (string, string, error) {
	key := spec.Name + " " + strings.Join(spec.Args, " ")
	if res, ok := f.results[key]; ok {
		return res.stdout, res.stderr, res.err
	}
	return "", "", errNotFound{}
}

// makeExecutable creates an empty executable file.
func makeExecutable(t *testing.T, path string) string {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		t.Fatalf("mkdir %s: %v", filepath.Dir(path), err)
	}
	if err := os.WriteFile(path, []byte("#!/bin/sh\n"), 0755); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
	return path
}

// darwinFixture builds a fake runner backed by real temp files.
type darwinFixture struct {
	runner   *fakeDarwinRunner
	homebrew string // Homebrew prefix or ""
	pathBin  string // directory reachable through fake PATH or ""
}

func newDarwinFixture(t *testing.T) (*darwinFixture, string) {
	t.Helper()
	f := &darwinFixture{
		runner: &fakeDarwinRunner{lookPath: map[string]string{}, results: map[string]runResult{}},
	}
	root := t.TempDir()
	return f, root
}

func (f *fakeDarwinRunner) addPATH(bin string) {
	f.pathBin = bin
	for _, cmd := range []string{"llvm-as", "llc", "clang", "llvm-config", "brew", "xcrun"} {
		if _, err := os.Stat(filepath.Join(bin, cmd)); err == nil {
			f.lookPath[cmd] = filepath.Join(bin, cmd)
		}
	}
}

func (f *fakeDarwinRunner) setBrewPrefix(prefix string) {
	f.results["/usr/local/bin/brew --prefix llvm"] = runResult{stdout: "  " + prefix + "\n\n"}
	f.lookPath["brew"] = "/usr/local/bin/brew"
}

func (f *fakeDarwinRunner) setXcrunFindClang(path string) {
	f.results["xcrun --find clang"] = runResult{stdout: path + "\n"}
}

// assertDarwinTools checks the discovered tool paths.
func assertDarwinTools(t *testing.T, got *LLVMTools, wantLlvmAs, wantLlc, wantClang string) {
	t.Helper()
	if got.LlvmAs != wantLlvmAs {
		t.Errorf("LlvmAs = %q, want %q", got.LlvmAs, wantLlvmAs)
	}
	if got.Llc != wantLlc {
		t.Errorf("Llc = %q, want %q", got.Llc, wantLlc)
	}
	if got.Clang != wantClang {
		t.Errorf("Clang = %q, want %q", got.Clang, wantClang)
	}
	if got.Lld != "" {
		t.Errorf("Darwin tools must not populate Lld, got %q", got.Lld)
	}
}

func TestDiscoverDarwinLLVMToolsConfiguredPathsTakePrecedence(t *testing.T) {
	f, root := newDarwinFixture(t)

	configuredBin := filepath.Join(root, "configured")
	llvmAs := makeExecutable(t, filepath.Join(configuredBin, "llvm-as"))
	llc := makeExecutable(t, filepath.Join(configuredBin, "llc"))
	clang := makeExecutable(t, filepath.Join(configuredBin, "clang"))

	// Also provide a PATH toolchain that must NOT win.
	pathBin := filepath.Join(root, "onpath")
	makeExecutable(t, filepath.Join(pathBin, "llvm-as"))
	makeExecutable(t, filepath.Join(pathBin, "llc"))
	makeExecutable(t, filepath.Join(pathBin, "clang"))
	f.runner.addPATH(pathBin)
	f.runner.setBrewPrefix(filepath.Join(root, "brewopt"))

	cfg := &config.Config{LLVM: config.LLVMConfig{LlvmAs: llvmAs, Llc: llc, Clang: clang}}
	tools, err := DiscoverDarwinLLVMTools(cfg, f.runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertDarwinTools(t, tools, llvmAs, llc, clang)
}

func TestDiscoverDarwinLLVMToolsPATHBeatsBrew(t *testing.T) {
	f, root := newDarwinFixture(t)

	pathBin := filepath.Join(root, "path-llvm", "bin")
	makeExecutable(t, filepath.Join(pathBin, "llvm-as"))
	makeExecutable(t, filepath.Join(pathBin, "llc"))
	f.runner.addPATH(pathBin)

	brewBin := filepath.Join(root, "brew-llvm", "bin")
	brewLlvmAs := makeExecutable(t, filepath.Join(brewBin, "llvm-as"))
	brewLlc := makeExecutable(t, filepath.Join(brewBin, "llc"))
	makeExecutable(t, filepath.Join(brewBin, "clang"))
	f.runner.setBrewPrefix(filepath.Dir(brewBin))

	tools, err := DiscoverDarwinLLVMTools(&config.Config{}, f.runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertDarwinTools(t, tools, filepath.Join(pathBin, "llvm-as"), filepath.Join(pathBin, "llc"), filepath.Join(brewBin, "clang"))
	_ = brewLlvmAs
	_ = brewLlc
}

func TestDiscoverDarwinLLVMToolsHomebrewFallback(t *testing.T) {
	f, root := newDarwinFixture(t)

	// No llvm-as/llc/llvm-config in PATH; only brew.
	brewBin := filepath.Join(root, "opt", "llvm", "bin")
	makeExecutable(t, filepath.Join(brewBin, "llvm-as"))
	makeExecutable(t, filepath.Join(brewBin, "llc"))
	clang := makeExecutable(t, filepath.Join(brewBin, "clang"))
	// No xcrun: clang must be found via the Homebrew prefix.
	f.runner.setBrewPrefix(filepath.Dir(brewBin))

	tools, err := DiscoverDarwinLLVMTools(&config.Config{}, f.runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertDarwinTools(t, tools,
		filepath.Join(brewBin, "llvm-as"),
		filepath.Join(brewBin, "llc"),
		clang)
}

func TestDiscoverDarwinLLVMToolsLLVMConfigFallback(t *testing.T) {
	f, root := newDarwinFixture(t)

	// llvm-config reachable through fake PATH; llvm-as/llc not.
	llvmConfigBin := filepath.Join(root, "toolchain", "bin")
	makeExecutable(t, filepath.Join(llvmConfigBin, "llvm-config"))

	// A different directory holding the actual tools (llvm-config --bindir).
	bindir := filepath.Join(root, "toolchain2", "bin")
	makeExecutable(t, filepath.Join(bindir, "llvm-as"))
	makeExecutable(t, filepath.Join(bindir, "llc"))
	makeExecutable(t, filepath.Join(bindir, "clang"))

	// Homebrew would win only if llvm-config were unavailable; a
	// different, valid toolchain lives there to prove precedence.
	brewBin := filepath.Join(root, "brew", "bin")
	makeExecutable(t, filepath.Join(brewBin, "llvm-as"))
	makeExecutable(t, filepath.Join(brewBin, "llc"))
	makeExecutable(t, filepath.Join(brewBin, "clang"))
	f.runner.setBrewPrefix(filepath.Dir(brewBin))

	f.runner.lookPath["llvm-config"] = filepath.Join(llvmConfigBin, "llvm-config")
	f.runner.results[filepath.Join(llvmConfigBin, "llvm-config")+" --bindir"] = runResult{stdout: bindir + "\n"}

	tools, err := DiscoverDarwinLLVMTools(&config.Config{}, f.runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// llvm-as/llc must come from the llvm-config --bindir directory;
	// clang follows its own precedence (here the Homebrew prefix).
	if tools.LlvmAs != filepath.Join(bindir, "llvm-as") || tools.Llc != filepath.Join(bindir, "llc") {
		t.Errorf("llvm-config --bindir must take precedence over brew for llvm-as/llc, got llvm-as=%q llc=%q", tools.LlvmAs, tools.Llc)
	}
	if tools.Clang == "" {
		t.Error("clang must still be resolved")
	}
}

func TestDiscoverDarwinLLVMToolsHomebrewOutputTrimmedAndValidated(t *testing.T) {
	f, root := newDarwinFixture(t)

	brewBin := filepath.Join(root, "opt", "llvm", "bin")
	makeExecutable(t, filepath.Join(brewBin, "llvm-as"))
	makeExecutable(t, filepath.Join(brewBin, "llc"))
	makeExecutable(t, filepath.Join(brewBin, "clang"))

	// Untrimmed, multi-line stdout with surrounding whitespace must be
	// trimmed and validated as an absolute prefix.
	f.runner.setBrewPrefix(filepath.Dir(brewBin))

	tools, err := DiscoverDarwinLLVMTools(&config.Config{}, f.runner)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	assertDarwinTools(t, tools, filepath.Join(brewBin, "llvm-as"), filepath.Join(brewBin, "llc"), filepath.Join(brewBin, "clang"))
}

// TestDiscoverDarwinLLVMToolsCanonicalPrefixWithoutBrewOnPath covers the
// regression where brew is installed (its opt prefix is populated) but the
// brew binary itself is not reachable through PATH. Discovery must probe
// the canonical Homebrew prefixes directly.
func TestDiscoverDarwinLLVMToolsCanonicalPrefixWithoutBrewOnPath(t *testing.T) {
	f, root := newDarwinFixture(t)

	// Populate a canonical Homebrew opt prefix; no brew, xcrun, or PATH
	// toolchain anywhere in the fixture.
	originalPrefixes := darwinStandardLLVMPrefixes
	optRoot := filepath.Join(root, "homebrew-root")
	darwinStandardLLVMPrefixes = []string{optRoot}
	t.Cleanup(func() { darwinStandardLLVMPrefixes = originalPrefixes })

	brewBin := filepath.Join(optRoot, "opt", "llvm", "bin")
	makeExecutable(t, filepath.Join(brewBin, "llvm-as"))
	makeExecutable(t, filepath.Join(brewBin, "llc"))
	clang := makeExecutable(t, filepath.Join(brewBin, "clang"))

	tools, err := DiscoverDarwinLLVMTools(&config.Config{}, f.runner)
	if err != nil {
		t.Fatalf("discovery must fall back to the canonical Homebrew opt prefix: %v", err)
	}
	assertDarwinTools(t, tools,
		filepath.Join(brewBin, "llvm-as"),
		filepath.Join(brewBin, "llc"),
		clang)
}

func TestDiscoverDarwinLLVMToolsMissingHomebrewIsNormalMiss(t *testing.T) {
	f, _ := newDarwinFixture(t)

	// Point the canonical prefixes at an empty location so the fixture
	// truly has no toolchain (CI macOS runners may have Homebrew LLVM).
	originalPrefixes := darwinStandardLLVMPrefixes
	darwinStandardLLVMPrefixes = []string{t.TempDir()}
	t.Cleanup(func() { darwinStandardLLVMPrefixes = originalPrefixes })

	// No brew anywhere: discovery must fail with installation guidance,
	// not crash.
	_, err := DiscoverDarwinLLVMTools(&config.Config{}, f.runner)
	if err == nil {
		t.Fatal("expected discovery failure with no toolchain")
	}
	if !strings.Contains(err.Error(), "brew install llvm") {
		t.Errorf("expected actionable guidance, got: %v", err)
	}
}

func TestDiscoverDarwinLLVMToolsBrewFormulaFailureHandled(t *testing.T) {
	f, root := newDarwinFixture(t)

	// Isolate canonical prefixes so CI macOS runners with preinstalled
	// Homebrew LLVM cannot satisfy discovery here.
	originalPrefixes := darwinStandardLLVMPrefixes
	darwinStandardLLVMPrefixes = []string{filepath.Join(root, "empty-homebrew")}
	t.Cleanup(func() { darwinStandardLLVMPrefixes = originalPrefixes })

	// brew exists but the llvm prefix is invalid (non-absolute output).
	pathBin := filepath.Join(root, "bin")
	makeExecutable(t, filepath.Join(pathBin, "brew"))
	f.runner.addPATH(pathBin)
	f.runner.results["brew --prefix llvm"] = runResult{stdout: "\n", err: errNotFound{}}

	_, err := DiscoverDarwinLLVMTools(&config.Config{}, f.runner)
	if err == nil {
		t.Fatal("expected discovery failure")
	}
	if !strings.Contains(err.Error(), "brew install llvm") {
		t.Errorf("expected actionable guidance, got: %v", err)
	}
}

func TestDiscoverDarwinLLVMToolsDoesNotRequireLld(t *testing.T) {
	f, root := newDarwinFixture(t)

	bindir := filepath.Join(root, "bin")
	makeExecutable(t, filepath.Join(bindir, "llvm-as"))
	makeExecutable(t, filepath.Join(bindir, "llc"))
	clang := makeExecutable(t, filepath.Join(bindir, "clang"))
	f.runner.addPATH(bindir)

	// No lld exists anywhere in the fixture.
	tools, err := DiscoverDarwinLLVMTools(&config.Config{}, f.runner)
	if err != nil {
		t.Fatalf("Darwin discovery must not require lld: %v", err)
	}
	if tools.Clang != clang {
		t.Errorf("Clang = %q, want %q", tools.Clang, clang)
	}
}

func TestFindLLVMToolsLinuxRequiresLld(t *testing.T) {
	root := t.TempDir()
	bin := filepath.Join(root, "bin")
	makeExecutable(t, filepath.Join(bin, "llvm-as"))
	makeExecutable(t, filepath.Join(bin, "llc"))
	// Deliberately no lld, llvm-as-N, llc-N, or lld-N.

	t.Setenv("PATH", bin)

	_, err := FindLLVMTools(&config.Config{}, platform.PlatformLinux)
	if err == nil {
		t.Fatal("Linux discovery must require lld")
	}
	if !strings.Contains(err.Error(), "Linker driver") {
		t.Errorf("expected the missing linker driver to be reported, got: %v", err)
	}

	// The same fixture satisfies Darwin, because clang (not lld) is the
	// required Darwin linker driver.
	makeExecutable(t, filepath.Join(bin, "clang"))
	darwinTools, err := FindLLVMTools(&config.Config{}, platform.PlatformDarwin)
	if err != nil {
		t.Fatalf("Darwin discovery must not require lld: %v", err)
	}
	if darwinTools.Clang != filepath.Join(bin, "clang") {
		t.Errorf("Clang = %q, want %q", darwinTools.Clang, filepath.Join(bin, "clang"))
	}
	if darwinTools.Lld != "" {
		t.Errorf("Darwin tools must not populate Lld, got %q", darwinTools.Lld)
	}
}

func TestLinkerCommand(t *testing.T) {
	if got, err := linkerCommand(platform.PlatformLinux); err != nil || got != "lld" {
		t.Errorf("Linux linker = %q, err = %v", got, err)
	}
	if got, err := linkerCommand(platform.PlatformDarwin); err != nil || got != "clang" {
		t.Errorf("Darwin linker = %q, err = %v", got, err)
	}
	if _, err := linkerCommand(platform.PlatformKind("windows")); err == nil {
		t.Error("unsupported platform must return an error")
	}
}

func TestDiscoverDarwinClangPrecedence(t *testing.T) {
	f, root := newDarwinFixture(t)

	// 1. Configured path wins.
	configured := makeExecutable(t, filepath.Join(root, "configured", "clang"))
	clang, err := DiscoverDarwinClang(&config.Config{LLVM: config.LLVMConfig{Clang: configured}}, f.runner)
	if err != nil || clang != configured {
		t.Errorf("configured clang: got %q err %v", clang, err)
	}

	// 2. xcrun beats PATH.
	xcrunClang := makeExecutable(t, filepath.Join(root, "xcrun", "clang"))
	pathClang := makeExecutable(t, filepath.Join(root, "path", "clang"))
	brewClang := makeExecutable(t, filepath.Join(root, "brew", "bin", "clang"))
	f.runner.addPATH(filepath.Join(root, "path"))
	f.runner.setXcrunFindClang(xcrunClang)
	clang, err = DiscoverDarwinClang(&config.Config{}, f.runner)
	if err != nil || clang != xcrunClang {
		t.Errorf("xcrun clang: got %q err %v", clang, err)
	}

	// 3. PATH beats brew.
	delete(f.runner.results, "xcrun --find clang")
	clang, err = DiscoverDarwinClang(&config.Config{}, f.runner)
	if err != nil || clang != pathClang {
		t.Errorf("PATH clang: got %q err %v", clang, err)
	}

	// 4. Homebrew prefix fallback.
	delete(f.runner.lookPath, "clang")
	makeExecutable(t, filepath.Join(root, "brewbin", "brew"))
	f.runner.lookPath["brew"] = filepath.Join(root, "brewbin", "brew")
	f.runner.setBrewPrefix(filepath.Join(root, "brew"))
	clang, err = DiscoverDarwinClang(&config.Config{}, f.runner)
	if err != nil || clang != brewClang {
		t.Errorf("brew clang: got %q err %v", clang, err)
	}

	// 5. Nothing available: actionable error.
	delete(f.runner.lookPath, "brew")
	_, err = DiscoverDarwinClang(&config.Config{}, f.runner)
	if err == nil || !strings.Contains(err.Error(), "xcode-select --install") {
		t.Errorf("expected actionable clang error, got: %v", err)
	}
}
