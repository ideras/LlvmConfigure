package platform

import (
	"fmt"
	"strings"
)

// IsDarwinTriple reports whether a triple represents a Darwin/macOS target.
func IsDarwinTriple(triple string) bool {
	t := strings.ToLower(triple)
	return strings.Contains(t, "darwin") ||
		strings.Contains(t, "macosx") ||
		strings.Contains(t, "macos") ||
		strings.Contains(t, "apple")
}

// TripleArch returns the architecture component (first component) of a triple.
func TripleArch(triple string) string {
	triple = strings.TrimSpace(triple)
	parts := strings.SplitN(triple, "-", 2)
	if len(parts) == 0 {
		return ""
	}
	return parts[0]
}

// DetectTargetTriple obtains the native target triple from the selected
// clang toolchain, which is the authority for the effective native target.
//
// The single documented command is `clang -print-target-triple`.
func DetectTargetTriple(clangPath string, runner CommandRunner) (string, error) {
	stdout, stderr, err := runner.Run(CommandSpec{Name: clangPath, Args: []string{"-print-target-triple"}})
	if err != nil {
		return "", fmt.Errorf("failed to detect the target triple with %s -print-target-triple: %s",
			clangPath, firstLine(stderr))
	}

	triple := firstLine(stdout)
	if triple == "" {
		return "", fmt.Errorf("%s -print-target-triple produced an empty target triple", clangPath)
	}

	if !IsDarwinTriple(triple) {
		return "", fmt.Errorf("clang %s does not target Darwin/macOS (reported %q); a macOS-capable clang is required", clangPath, triple)
	}

	return triple, nil
}

// ValidateStoredTarget compares a stored target triple against a freshly
// detected native target triple. A stored target is authoritative for
// reproducibility but must be validated before use; cross-compilation is
// not supported, so a mismatched architecture is rejected explicitly.
func ValidateStoredTarget(stored, detected string) error {
	if stored == detected {
		return nil
	}

	storedArch, detectedArch := TripleArch(stored), TripleArch(detected)
	if storedArch != detectedArch {
		return fmt.Errorf(
			"configured target %s does not match the current native clang target %s. "+
				"Cross-compilation is not supported: only native builds are supported. "+
				"Run --scan-llvm to refresh it",
			stored, detected)
	}

	return fmt.Errorf(
		"configured target %s is stale (the current native clang target is %s). "+
			"Run --scan-llvm to refresh it",
		stored, detected)
}

// macosxComponent returns the index of the macOS version component of a
// triple (e.g. "macosx14.0.0" in "arm64-apple-macosx14.0.0"), or -1.
func macosxComponent(triple string) int {
	parts := strings.Split(triple, "-")
	for i, part := range parts {
		if part == "macosx" || strings.HasPrefix(part, "macosx") {
			return i
		}
	}
	return -1
}

// ValidateMacosxVersion validates a macOS deployment target version.
// Accepted spellings are "X.Y" and "X.Y.Z" with numeric components.
func ValidateMacosxVersion(version string) error {
	parts := strings.Split(version, ".")
	if len(parts) != 2 && len(parts) != 3 {
		return fmt.Errorf("invalid macOS deployment target %q: expected a version such as 14.0", version)
	}
	for _, part := range parts {
		if part == "" {
			return fmt.Errorf("invalid macOS deployment target %q: expected a version such as 14.0", version)
		}
		for _, r := range part {
			if r < '0' || r > '9' {
				return fmt.Errorf("invalid macOS deployment target %q: expected a version such as 14.0", version)
			}
		}
	}
	return nil
}

// CodegenTriple centralizes the transformation of a stored/detected target
// triple plus an optional deployment target into the code-generation triple
// passed to llc (-mtriple) and clang (-target), plus the
// -mmacosx-version-min flag value ("" when no deployment target is set).
//
// When a deployment target is configured it is validated and, consistently,
// normalized into the triple's macosx component, so llc code generation and
// clang link metadata always agree.
func CodegenTriple(triple, deploymentTarget string) (codegenTriple, versionMin string, err error) {
	triple = strings.TrimSpace(triple)
	if triple == "" {
		return "", "", fmt.Errorf("target triple is empty")
	}

	if deploymentTarget == "" {
		return triple, "", nil
	}

	if err := ValidateMacosxVersion(deploymentTarget); err != nil {
		return "", "", err
	}

	parts := strings.Split(triple, "-")
	idx := macosxComponent(triple)
	if idx < 0 {
		return "", "", fmt.Errorf(
			"target triple %q has no macOS version component; cannot apply deployment target %q",
			triple, deploymentTarget)
	}

	parts[idx] = "macosx" + deploymentTarget
	return strings.Join(parts, "-"), deploymentTarget, nil
}

// ValidateLLCTriple verifies that the selected llc accepts the target
// triple by feeding it a minimal LLVM module with -mtriple. Failures carry
// the llc command and trimmed stderr so they read as actionable
// source/toolchain errors rather than generic linker failures.
func ValidateLLCTriple(llcPath, triple string, runner CommandRunner) error {
	const minimalModule = "define i32 @main() {\n  ret i32 0\n}\n"

	stdout, stderr, err := runner.Run(CommandSpec{
		Name:  llcPath,
		Args:  []string{"-mtriple=" + triple, "-filetype=obj", "-o", "/dev/null"},
		Stdin: minimalModule,
	})
	if err != nil {
		detail := firstLine(stderr)
		if detail == "" {
			detail = firstLine(stdout)
		}
		if detail == "" {
			detail = err.Error()
		}
		return fmt.Errorf("llc %s rejected the target triple %s: %s", llcPath, triple, detail)
	}
	return nil
}

// firstLine returns the first non-empty line of s, trimmed.
func firstLine(s string) string {
	for _, line := range strings.Split(s, "\n") {
		line = strings.TrimSpace(line)
		if line != "" {
			return line
		}
	}
	return ""
}
