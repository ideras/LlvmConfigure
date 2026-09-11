package platform

import (
	"errors"
	"strings"
	"testing"
)

type fakeRunner struct {
	lookPath    map[string]string
	lookPathErr map[string]bool
	runResults  map[string]runResult
	catchAll    func(spec CommandSpec) (runResult, bool)
}

type runResult struct {
	stdout string
	stderr string
	err    error
}

func (f *fakeRunner) LookPath(file string) (string, error) {
	if f.lookPathErr[file] {
		return "", errors.New("not found")
	}
	if path, ok := f.lookPath[file]; ok {
		return path, nil
	}
	return "", errors.New("not found")
}

func (f *fakeRunner) Run(spec CommandSpec) (string, string, error) {
	key := spec.Name
	for _, arg := range spec.Args {
		key += " " + arg
	}
	if res, ok := f.runResults[key]; ok {
		return res.stdout, res.stderr, res.err
	}
	if f.catchAll != nil {
		if res, ok := f.catchAll(spec); ok {
			return res.stdout, res.stderr, res.err
		}
	}
	return "", "", errors.New("unexpected command: " + key)
}

func newFakeLLCRunner() *fakeRunner {
	r := &fakeRunner{runResults: map[string]runResult{}}
	// Register a catch-all llc success matched by binary name suffix.
	r.catchAll = func(spec CommandSpec) (runResult, bool) {
		if strings.HasSuffix(spec.Name, "llc") {
			return runResult{}, true
		}
		return runResult{}, false
	}
	return r
}

func TestDetectSupportedPlatforms(t *testing.T) {
	plat, err := Detect()
	if err != nil {
		t.Fatalf("Detect on a supported host: %v", err)
	}
	if plat != PlatformLinux && plat != PlatformDarwin {
		t.Errorf("unexpected platform %q", plat)
	}
}

func TestIsDarwinTriple(t *testing.T) {
	for _, triple := range []string{"arm64-apple-macosx14.0.0", "x86_64-apple-darwin", "arm64-apple-macos13.3"} {
		if !IsDarwinTriple(triple) {
			t.Errorf("expected %q to be recognized as Darwin", triple)
		}
	}
	for _, triple := range []string{"x86_64-pc-linux-gnu", "aarch64-unknown-linux-gnu", ""} {
		if IsDarwinTriple(triple) {
			t.Errorf("expected %q NOT to be recognized as Darwin", triple)
		}
	}
}

func TestValidateMacosxVersion(t *testing.T) {
	for _, version := range []string{"14.0", "13.3", "10.15.7"} {
		if err := ValidateMacosxVersion(version); err != nil {
			t.Errorf("expected %q to be valid: %v", version, err)
		}
	}
	for _, version := range []string{"", "14", "v14.0", "14.0.0.0", "14..0", "fourteen.0", "-14.0"} {
		if err := ValidateMacosxVersion(version); err == nil {
			t.Errorf("expected %q to be rejected", version)
		}
	}
}

func TestCodegenTriple(t *testing.T) {
	tests := []struct {
		name             string
		triple           string
		deploymentTarget string
		wantTriple       string
		wantMin          string
		wantErr          bool
	}{
		{
			name:             "no deployment target passes triple through",
			triple:           "arm64-apple-macosx14.0.0",
			deploymentTarget: "",
			wantTriple:       "arm64-apple-macosx14.0.0",
			wantMin:          "",
		},
		{
			name:             "deployment target normalizes the macosx component",
			triple:           "arm64-apple-macosx14.0.0",
			deploymentTarget: "13.0",
			wantTriple:       "arm64-apple-macosx13.0",
			wantMin:          "13.0",
		},
		{
			name:             "triple without version gets deployment target",
			triple:           "arm64-apple-macosx",
			deploymentTarget: "14.0",
			wantTriple:       "arm64-apple-macosx14.0",
			wantMin:          "14.0",
		},
		{
			name:             "invalid deployment target rejected",
			triple:           "arm64-apple-macosx14.0.0",
			deploymentTarget: "not-a-version",
			wantErr:          true,
		},
		{
			name:             "triple without macosx component rejected",
			triple:           "arm64-apple-darwin",
			deploymentTarget: "14.0",
			wantErr:          true,
		},
		{
			name:             "empty triple rejected",
			triple:           "",
			deploymentTarget: "",
			wantErr:          true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			gotTriple, gotMin, err := CodegenTriple(tc.triple, tc.deploymentTarget)
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got triple %q min %q", gotTriple, gotMin)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if gotTriple != tc.wantTriple {
				t.Errorf("codegen triple = %q, want %q", gotTriple, tc.wantTriple)
			}
			if gotMin != tc.wantMin {
				t.Errorf("version min = %q, want %q", gotMin, tc.wantMin)
			}
		})
	}
}

func TestValidateStoredTarget(t *testing.T) {
	if err := ValidateStoredTarget("arm64-apple-macosx14.0.0", "arm64-apple-macosx14.0.0"); err != nil {
		t.Errorf("identical triples should validate: %v", err)
	}

	err := ValidateStoredTarget("x86_64-apple-macosx13.0.0", "arm64-apple-macosx14.0.0")
	if err == nil {
		t.Fatal("architectural mismatch must be rejected")
	}
	msg := err.Error()
	for _, want := range []string{"x86_64-apple-macosx13.0.0", "arm64-apple-macosx14.0.0", "--scan-llvm", "native"} {
		if !strings.Contains(msg, want) {
			t.Errorf("arch mismatch error should mention %q, got: %s", want, msg)
		}
	}

	err = ValidateStoredTarget("arm64-apple-macosx13.0.0", "arm64-apple-macosx14.0.0")
	if err == nil {
		t.Fatal("stale triple (same arch, different OS version) must be rejected")
	}
	if !strings.Contains(err.Error(), "--scan-llvm") {
		t.Errorf("stale triple error should recommend --scan-llvm, got: %v", err)
	}
}

func TestDetectTargetTriple(t *testing.T) {
	clang := "/usr/bin/clang"

	t.Run("success", func(t *testing.T) {
		runner := &fakeRunner{runResults: map[string]runResult{
			"/usr/bin/clang -print-target-triple": {stdout: "arm64-apple-macosx14.0.0\n"},
		}}
		triple, err := DetectTargetTriple(clang, runner)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if triple != "arm64-apple-macosx14.0.0" {
			t.Errorf("triple = %q", triple)
		}
	})

	t.Run("empty output rejected", func(t *testing.T) {
		runner := &fakeRunner{runResults: map[string]runResult{
			"/usr/bin/clang -print-target-triple": {stdout: "\n"},
		}}
		_, err := DetectTargetTriple(clang, runner)
		if err == nil || !strings.Contains(err.Error(), "empty target triple") {
			t.Errorf("expected empty-triple error, got: %v", err)
		}
	})

	t.Run("non-darwin output rejected", func(t *testing.T) {
		runner := &fakeRunner{runResults: map[string]runResult{
			"/usr/bin/clang -print-target-triple": {stdout: "x86_64-pc-linux-gnu\n"},
		}}
		_, err := DetectTargetTriple(clang, runner)
		if err == nil || !strings.Contains(err.Error(), "Darwin") {
			t.Errorf("expected non-Darwin rejection, got: %v", err)
		}
	})

	t.Run("clang failure carries stderr", func(t *testing.T) {
		runner := &fakeRunner{runResults: map[string]runResult{
			"/usr/bin/clang -print-target-triple": {stderr: "no SDK", err: errors.New("exit 1")},
		}}
		_, err := DetectTargetTriple(clang, runner)
		if err == nil || !strings.Contains(err.Error(), "no SDK") {
			t.Errorf("expected error carrying trimmed stderr, got: %v", err)
		}
	})
}

func TestValidateLLCTriple(t *testing.T) {
	runner := newFakeLLCRunner()
	if err := ValidateLLCTriple("/opt/homebrew/opt/llvm/bin/llc", "arm64-apple-macosx14.0.0", runner); err != nil {
		t.Fatalf("expected llc to accept a valid triple: %v", err)
	}

	failing := &fakeRunner{runResults: map[string]runResult{
		"/bad/llc -mtriple=arm64-apple-macosx14.0.0 -filetype=obj -o /dev/null": {
			stderr: "invalid target triple\nmore detail",
			err:    errors.New("exit 1"),
		},
	}}
	if err := ValidateLLCTriple("/bad/llc", "arm64-apple-macosx14.0.0", failing); err == nil {
		t.Fatal("expected llc rejection error")
	} else if !strings.Contains(err.Error(), "invalid target triple") {
		t.Errorf("error should carry trimmed llc stderr, got: %v", err)
	}
}

func TestValidateMacSDK(t *testing.T) {
	runner := &fakeRunner{runResults: map[string]runResult{
		"xcrun --sdk macosx --show-sdk-path": {stdout: "/Applications/Xcode.app/Contents/Developer/Platforms/MacOSX.platform/Developer/SDKs/MacOSX.sdk\n"},
	}}
	if err := ValidateMacSDK(runner); err != nil {
		t.Fatalf("expected SDK validation success: %v", err)
	}

	runner = &fakeRunner{runResults: map[string]runResult{
		"xcrun --sdk macosx --show-sdk-path": {stderr: "xcrun: error: unable to find utility", err: errors.New("exit 1")},
	}}
	err := ValidateMacSDK(runner)
	if err == nil {
		t.Fatal("expected SDK validation failure")
	}
	for _, want := range []string{"xcode-select --install", "Xcode Command Line Tools"} {
		if !strings.Contains(err.Error(), want) {
			t.Errorf("SDK error should contain %q, got: %v", want, err)
		}
	}
}
