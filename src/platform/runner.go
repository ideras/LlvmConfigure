package platform

import (
	"bytes"
	"os/exec"
)

// CommandSpec describes one external command invocation.
// Stdin may be empty.
type CommandSpec struct {
	Name  string
	Args  []string
	Stdin string
}

// CommandRunner abstracts process lookup and execution so discovery can be
// unit-tested without Homebrew, Xcode, or macOS itself.
type CommandRunner interface {
	// LookPath searches for an executable in PATH (see exec.LookPath).
	LookPath(file string) (string, error)
	// Run executes a command and returns trimmed-able stdout/stderr.
	Run(spec CommandSpec) (stdout string, stderr string, err error)
}

// ExecCommandRunner is the production CommandRunner backed by os/exec.
// It never invokes a shell.
type ExecCommandRunner struct{}

// LookPath implements CommandRunner.
func (ExecCommandRunner) LookPath(file string) (string, error) {
	return exec.LookPath(file)
}

// Run implements CommandRunner.
func (ExecCommandRunner) Run(spec CommandSpec) (string, string, error) {
	cmd := exec.Command(spec.Name, spec.Args...)
	if spec.Stdin != "" {
		cmd.Stdin = bytes.NewBufferString(spec.Stdin)
	}
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	return stdout.String(), stderr.String(), err
}

// DefaultRunner is the production command runner.
var DefaultRunner CommandRunner = ExecCommandRunner{}
