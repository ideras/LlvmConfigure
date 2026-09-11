package platform

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func writeLLFile(t *testing.T, dir, name, content string) string {
	t.Helper()
	path := filepath.Join(dir, name)
	if err := os.WriteFile(path, []byte(content), 0644); err != nil {
		t.Fatalf("write %s: %v", name, err)
	}
	return path
}

func TestFindConflictingSourceTriples(t *testing.T) {
	dir := t.TempDir()
	target := "arm64-apple-macosx14.0.0"

	conflicting := writeLLFile(t, dir, "conflicting.ll",
		"; ModuleID = 'x'\ntarget datalayout = \"e\"\ntarget triple = \"x86_64-pc-linux-gnu\"\n\ndefine i32 @main() {\n  ret i32 0\n}\n")
	matching := writeLLFile(t, dir, "matching.ll",
		"target triple = \"arm64-apple-macosx14.0.0\"\n\ndefine i32 @main() {\n  ret i32 0\n}\n")
	noTriple := writeLLFile(t, dir, "notriple.ll",
		"define i32 @main() {\n  ret i32 0\n}\n")

	warnings := FindConflictingSourceTriples([]string{conflicting, matching, noTriple}, target)
	if len(warnings) != 1 {
		t.Fatalf("expected exactly one warning, got %d: %v", len(warnings), warnings)
	}
	if !strings.Contains(warnings[0], "conflicting.ll") {
		t.Errorf("warning should name the conflicting file, got: %s", warnings[0])
	}
	if !strings.Contains(warnings[0], "x86_64-pc-linux-gnu") || !strings.Contains(warnings[0], target) {
		t.Errorf("warning should carry both triples, got: %s", warnings[0])
	}

	// A missing/unreadable file must not panic or produce warnings.
	warnings = FindConflictingSourceTriples([]string{filepath.Join(dir, "missing.ll")}, target)
	if len(warnings) != 0 {
		t.Errorf("expected no warnings for unreadable file, got: %v", warnings)
	}
}
