package tools

import (
	"fmt"
	"path/filepath"
	"strings"

	"llvm-configure/config"
	"llvm-configure/platform"
)

// findDarwinSystemTool searches the (injectable) PATH for command,
// including version-suffixed variants such as llvm-as-20.
func findDarwinSystemTool(command string, runner platform.CommandRunner) (string, bool) {
	for version := 20; version >= 10; version-- {
		if path, err := runner.LookPath(fmt.Sprintf("%s-%d", command, version)); err == nil && path != "" {
			return path, true
		}
	}

	path, err := runner.LookPath(command)
	return path, err == nil && path != ""
}

// DiscoverDarwinLLVMTools finds llvm-as/llc/clang on macOS following the documented precedence.
//
//  1. Valid explicit paths in configuration.
//  2. A coherent toolchain already available through PATH.
//  3. `llvm-config --bindir`, when llvm-config is available.
//  4. `brew --prefix llvm`, when Homebrew is available.
//  5. Fail with installation guidance.
//
// llvm-as and llc are discovered from the same bin directory wherever
// possible so incompatible LLVM major versions are not combined.
// Homebrew LLVM is keg-only and current Homebrew LLVM does not provide
// lld, so Darwin never requires it.
func DiscoverDarwinLLVMTools(cfg *config.Config, runner platform.CommandRunner) (*LLVMTools, error) {
	llvmAs, llvmAsConfigured := validConfiguredPath(cfg.LLVM.LlvmAs)
	llc, llcConfigured := validConfiguredPath(cfg.LLVM.Llc)

	// 1. Configuration
	if llvmAsConfigured && llcConfigured {
		return resolveDarwinPair(llvmAs, llc, cfg, runner)
	}

	// 2-4. Coherent toolchain directories, in precedence order.
	for _, dir := range darwinCandidateDirs(cfg, runner) {
		if a, l, ok := toolsInDir(dir); ok {
			if !llvmAsConfigured {
				llvmAs = a
				llvmAsConfigured = true
			}
			if !llcConfigured {
				llc = l
				llcConfigured = true
			}
			return resolveDarwinPair(llvmAs, llc, cfg, runner)
		}
	}

	// 5. Independent PATH fallback (incoherent but usable).
	if !llvmAsConfigured {
		if path, ok := findDarwinSystemTool("llvm-as", runner); ok {
			llvmAs, llvmAsConfigured = path, true
		}
	}
	if !llcConfigured {
		if path, ok := findDarwinSystemTool("llc", runner); ok {
			llc, llcConfigured = path, true
		}
	}
	if llvmAsConfigured && llcConfigured {
		return resolveDarwinPair(llvmAs, llc, cfg, runner)
	}

	return nil, fmt.Errorf(
		"Homebrew LLVM was not found. Install it with: brew install llvm\n" +
			"(or run llvm-configure --scan-llvm after adding it to PATH)")
}

// resolveDarwinPair combines discovered llvm-as/llc paths with a clang
// linker driver, which is required on Darwin.
func resolveDarwinPair(llvmAs, llc string, cfg *config.Config, runner platform.CommandRunner) (*LLVMTools, error) {
	clang, err := DiscoverDarwinClang(cfg, runner)
	if err != nil {
		return nil, err
	}
	return &LLVMTools{LlvmAs: llvmAs, Llc: llc, Clang: clang}, nil
}

// darwinCandidateDirs returns the bin directories to probe for a coherent
// llvm-as/llc toolchain, in precedence order: PATH, llvm-config --bindir,
// the Homebrew LLVM prefix, and finally the canonical Homebrew opt
// prefixes probed directly.
func darwinCandidateDirs(cfg *config.Config, runner platform.CommandRunner) []string {
	var dirs []string

	// 2. Directories already reachable through PATH.
	for _, command := range []string{"llvm-as", "llc"} {
		if path, err := runner.LookPath(command); err == nil && path != "" {
			dirs = append(dirs, filepath.Dir(path))
		}
	}

	// 3. llvm-config --bindir.
	if path, err := runner.LookPath("llvm-config"); err == nil && path != "" {
		if out, _, err := runner.Run(platform.CommandSpec{Name: path, Args: []string{"--bindir"}}); err == nil {
			if dir := trimAndValidateDir(out); dir != "" {
				dirs = append(dirs, dir)
			}
		}
	}

	// 4. brew --prefix llvm. Homebrew absence or a missing formula is a
	// normal discovery miss, not a crash.
	if prefix, ok := BrewLLVMPrefix(runner); ok {
		dirs = append(dirs, filepath.Join(prefix, "bin"))
	}

	// 5. Canonical Homebrew prefixes probed directly, so discovery does
	// not depend on brew itself being on PATH (brew is installed in
	// /usr/local/bin on Intel Macs and /opt/homebrew/bin on Apple
	// Silicon, and llvm is keg-only, so these opt paths are stable).
	for _, prefix := range darwinStandardLLVMPrefixes {
		dirs = append(dirs, filepath.Join(prefix, "opt", "llvm", "bin"))
	}

	return dirs
}

// darwinStandardLLVMPrefixes holds the canonical Homebrew installation
// prefixes. It is a variable so tests can point it at temporary
// directories; in production it covers both Apple Silicon (/opt/homebrew)
// and Intel (/usr/local) layouts.
var darwinStandardLLVMPrefixes = []string{"/opt/homebrew", "/usr/local"}

// toolsInDir returns the llvm-as/llc pair in dir when both exist there as
// executable files.
func toolsInDir(dir string) (llvmAs, llc string, ok bool) {
	llvmAsPath := filepath.Join(dir, "llvm-as")
	llcPath := filepath.Join(dir, "llc")
	if isExecutableFile(llvmAsPath) && isExecutableFile(llcPath) {
		return llvmAsPath, llcPath, true
	}
	return "", "", false
}

// trimAndValidateDir trims command output and requires an absolute path.
func trimAndValidateDir(output string) string {
	dir := strings.TrimSpace(output)
	if dir == "" || !filepath.IsAbs(dir) {
		return ""
	}
	return dir
}

// validConfiguredPath returns the path when it is an executable file.
func validConfiguredPath(path string) (string, bool) {
	if isExecutableFile(path) {
		return path, true
	}
	return "", false
}

// BrewLLVMPrefix executes `brew --prefix llvm` directly (never through a
// shell), trims and validates stdout, and returns the stable Homebrew opt
// prefix. A missing brew or missing formula is a normal discovery miss.
func BrewLLVMPrefix(runner platform.CommandRunner) (string, bool) {
	brewPath, err := runner.LookPath("brew")
	if err != nil || brewPath == "" {
		return "", false
	}

	stdout, _, err := runner.Run(platform.CommandSpec{Name: brewPath, Args: []string{"--prefix", "llvm"}})
	if err != nil {
		return "", false
	}

	prefix := strings.TrimSpace(stdout)
	if prefix == "" || !filepath.IsAbs(prefix) {
		return "", false
	}

	// The prefix returned by `brew --prefix llvm` is the stable opt path
	// (e.g. /opt/homebrew/opt/llvm), not a versioned Cellar path.
	return prefix, true
}

// DiscoverDarwinClang resolves the clang linker driver on Darwin.
// Apple clang is acceptable as the linker driver even when paired with
// Homebrew llc output: the linker consumes Mach-O object files and does
// not need to match the llvm-as/llc major version.
//
// Precedence: configured path, `xcrun --find clang`, PATH, and finally
// <brew-llvm-prefix>/bin/clang.
func DiscoverDarwinClang(cfg *config.Config, runner platform.CommandRunner) (string, error) {
	// 1. Configured clang path.
	if path, ok := validConfiguredPath(cfg.LLVM.Clang); ok {
		return path, nil
	}

	// 2. xcrun --find clang.
	if stdout, _, err := runner.Run(platform.CommandSpec{Name: "xcrun", Args: []string{"--find", "clang"}}); err == nil {
		path := strings.TrimSpace(stdout)
		if filepath.IsAbs(path) && isExecutableFile(path) {
			return path, nil
		}
	}

	// 3. clang from PATH.
	if path, err := runner.LookPath("clang"); err == nil && path != "" {
		return path, nil
	}

	// 4. <brew-llvm-prefix>/bin/clang, then the canonical Homebrew opt
	// prefixes probed directly.
	candidates := []string{}
	if prefix, ok := BrewLLVMPrefix(runner); ok {
		candidates = append(candidates, filepath.Join(prefix, "bin"))
	}
	for _, stdPrefix := range darwinStandardLLVMPrefixes {
		candidates = append(candidates, filepath.Join(stdPrefix, "opt", "llvm", "bin"))
	}
	for _, dir := range candidates {
		path := filepath.Join(dir, "clang")
		if isExecutableFile(path) {
			return path, nil
		}
	}

	return "", fmt.Errorf(
		"clang was not found. Install Xcode Command Line Tools with:\n" +
			"  xcode-select --install\n" +
			"clang is the required Darwin linker driver; raw lld is not needed on macOS")
}
