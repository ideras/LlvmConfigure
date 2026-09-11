package platform

import (
	"fmt"
	"strings"
)

// MacSDKPath returns the active macOS SDK path via
// `xcrun --sdk macosx --show-sdk-path`, trimmed and validated. The path
// is resolved on demand and is intentionally NOT persisted anywhere:
// Xcode updates can invalidate it.
func MacSDKPath(runner CommandRunner) (string, error) {
	stdout, stderr, err := runner.Run(CommandSpec{
		Name: "xcrun",
		Args: []string{"--sdk", "macosx", "--show-sdk-path"},
	})
	if err != nil {
		detail := firstLine(stderr)
		if detail == "" {
			detail = err.Error()
		}
		return "", fmt.Errorf(
			"a macOS SDK is unavailable. Install Xcode Command Line Tools with:\n"+
				"  xcode-select --install\n"+
				"(xcrun --sdk macosx --show-sdk-path failed: %s)", detail)
	}

	sdkPath := firstLine(stdout)
	if sdkPath == "" {
		return "", fmt.Errorf("xcrun --sdk macosx --show-sdk-path returned an empty SDK path")
	}
	return sdkPath, nil
}

// ValidateMacSDK checks that a macOS SDK is available through xcrun.
// Failures return actionable guidance pointing at Xcode Command Line Tools.
func ValidateMacSDK(runner CommandRunner) error {
	_, err := MacSDKPath(runner)
	return err
}

// FindConflictingSourceTriples performs a focused (non-parsing) scan of LLVM
// IR source files for an explicit `target triple = "..."` directive that
// conflicts with the selected target. It returns one warning per conflicting
// file; llc -mtriple will override the directive, and the warning makes the
// override explicit. It is not a full LLVM IR parser.
func FindConflictingSourceTriples(sourceFiles []string, targetTriple string) []string {
	var warnings []string

	for _, file := range sourceFiles {
		value, found := scanExplicitTargetTriple(file)
		if !found {
			continue
		}
		if value == targetTriple {
			continue
		}
		warnings = append(warnings, fmt.Sprintf(
			"%s declares target triple %q, which does not match the selected target %q; llc -mtriple will override it",
			file, value, targetTriple))
	}

	return warnings
}

// explicitTriplePattern is matched per line so only top-level IR directives
// (usually in the file header) are considered.
const explicitTriplePattern = `target triple = "`

func scanExplicitTargetTriple(path string) (string, bool) {
	data, err := readSourceFile(path, maxIRScanBytes)
	if err != nil {
		// Unreadable sources are reported elsewhere; missing an IR
		// warning must not break configuration.
		return "", false
	}

	for _, line := range strings.Split(string(data), "\n") {
		idx := strings.Index(line, explicitTriplePattern)
		if idx < 0 {
			continue
		}
		rest := line[idx+len(explicitTriplePattern):]
		end := strings.Index(rest, `"`)
		if end < 0 {
			return "", false
		}
		return strings.TrimSpace(rest[:end]), true
	}
	return "", false
}

// maxIRScanBytes bounds how much of an .ll file is scanned for the
// target triple directive; the directive appears near the top of any
// module llvm-as produces.
const maxIRScanBytes = 1 << 20
