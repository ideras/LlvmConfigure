package config

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestLoadOldConfigWithoutNewFields ensures existing configuration files,
// which predate the clang and target sections, still deserialize.
func TestLoadOldConfigWithoutNewFields(t *testing.T) {
	oldConfig := `{
  "llvm": {
    "llvm_as": "/usr/bin/llvm-as",
    "llc": "/usr/bin/llc",
    "lld": "/usr/bin/lld"
  },
  "libc": {
    "use_musl": false,
    "path": "/usr/lib/x86_64-linux-gnu",
    "dyn_linker_path": "/lib64/ld-linux-x86-64.so.2"
  }
}`

	path := filepath.Join(t.TempDir(), "config.json")
	if err := os.WriteFile(path, []byte(oldConfig), 0644); err != nil {
		t.Fatalf("write config: %v", err)
	}

	cfg, err := LoadConfigFrom(path)
	if err != nil {
		t.Fatalf("LoadConfigFrom: %v", err)
	}

	if cfg.LLVM.LlvmAs != "/usr/bin/llvm-as" || cfg.LLVM.Llc != "/usr/bin/llc" || cfg.LLVM.Lld != "/usr/bin/lld" {
		t.Errorf("LLVM tools not preserved: %+v", cfg.LLVM)
	}
	if cfg.LLVM.Clang != "" {
		t.Errorf("expected empty Clang in old config, got %q", cfg.LLVM.Clang)
	}
	if cfg.Target.Triple != "" || cfg.Target.DeploymentTarget != "" || cfg.Target.DetectedBy != "" {
		t.Errorf("expected zero target in old config, got %+v", cfg.Target)
	}
	if cfg.LibC.UseMusl || cfg.LibC.Path != "/usr/lib/x86_64-linux-gnu" || cfg.LibC.DynLinkerPath != "/lib64/ld-linux-x86-64.so.2" {
		t.Errorf("LibC not preserved: %+v", cfg.LibC)
	}
}

// TestConfigRoundTripClangAndTarget verifies the new clang and target
// fields round-trip through the JSON schema with stable lowercase,
// underscore-separated keys.
func TestConfigRoundTripClangAndTarget(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")

	cfg := &Config{
		LLVM: LLVMConfig{
			LlvmAs: "/opt/homebrew/opt/llvm/bin/llvm-as",
			Llc:    "/opt/homebrew/opt/llvm/bin/llc",
			Lld:    "",
			Clang:  "/usr/bin/clang",
		},
		Target: TargetConfig{
			Triple:           "arm64-apple-macosx14.0.0",
			DeploymentTarget: "14.0",
			DetectedBy:       "/usr/bin/clang",
		},
	}

	if err := SaveConfigTo(path, cfg); err != nil {
		t.Fatalf("SaveConfigTo: %v", err)
	}

	loaded, err := LoadConfigFrom(path)
	if err != nil {
		t.Fatalf("LoadConfigFrom: %v", err)
	}

	if loaded.LLVM != cfg.LLVM {
		t.Errorf("LLVM round-trip mismatch: got %+v, want %+v", loaded.LLVM, cfg.LLVM)
	}
	if loaded.Target != cfg.Target {
		t.Errorf("Target round-trip mismatch: got %+v, want %+v", loaded.Target, cfg.Target)
	}
}

// TestConfigJSONKeysAreStable locks the persisted JSON key names so
// external tooling and older/newer binaries remain compatible.
func TestConfigJSONKeysAreStable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "config.json")
	if err := SaveConfigTo(path, &Config{
		LLVM:   LLVMConfig{LlvmAs: "a", Llc: "b", Lld: "c", Clang: "d"},
		Target: TargetConfig{Triple: "t", DeploymentTarget: "d", DetectedBy: "x"},
	}); err != nil {
		t.Fatalf("SaveConfigTo: %v", err)
	}

	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read config: %v", err)
	}

	for _, key := range []string{`"llvm_as"`, `"llc"`, `"lld"`, `"clang"`, `"triple"`, `"deployment_target"`, `"detected_by"`, `"use_musl"`, `"path"`, `"dyn_linker_path"`} {
		if !strings.Contains(string(data), key) {
			t.Errorf("expected JSON key %s in config output:\n%s", key, data)
		}
	}
}
