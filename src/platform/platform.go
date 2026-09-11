// Package platform represents the build platform explicitly instead of
// scattering runtime.GOOS branches through the codebase.
//
// The central invariant is that a target triple, object format, and linker
// driver must agree:
//
//	Linux  -> ELF objects    -> GNU-flavor LLD and Linux libc strategy
//	Darwin -> Mach-O objects -> clang and the macOS SDK/libSystem strategy
//
// A build must fail during configuration, with an actionable message, if
// that invariant cannot be established.
package platform

import (
	"fmt"
	"runtime"
)

// PlatformKind identifies the supported native build platforms.
type PlatformKind string

const (
	PlatformLinux  PlatformKind = "linux"
	PlatformDarwin PlatformKind = "darwin"
)

// Target describes the native target of the build.
//
// Triple is the LLVM target triple reported by the selected clang
// toolchain (clang -print-target-triple); the clang toolchain — not
// runtime.GOARCH — is the authority for the effective native target,
// because a Go binary may be running under Rosetta.
//
// DeploymentTarget is a separate compatibility policy. An empty value
// means "use the toolchain's detected/default macOS target".
type Target struct {
	Platform         PlatformKind
	Triple           string
	DeploymentTarget string
}

// Detect returns the native build platform of the running executable.
// Unsupported operating systems return an explicit error rather than
// accidentally entering the Linux path.
func Detect() (PlatformKind, error) {
	switch runtime.GOOS {
	case "linux":
		return PlatformLinux, nil
	case "darwin":
		return PlatformDarwin, nil
	default:
		return "", fmt.Errorf("unsupported operating system %q: this tool supports native builds on Linux and macOS only", runtime.GOOS)
	}
}

// String implements fmt.Stringer.
func (p PlatformKind) String() string { return string(p) }
