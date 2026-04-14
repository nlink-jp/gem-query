package shell

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/nlink-jp/gem-query/internal/output"
	"github.com/nlink-jp/gem-query/internal/query"
)

// jvizState manages the persistent jviz integration.
type jvizState struct {
	binPath string // explicit path to jviz binary
	active  bool
	tmpFile string
	proc    *os.Process
	port    string
}

// start activates live jviz mode.
func (j *jvizState) start(port string) error {
	if j.binPath == "" {
		return fmt.Errorf("jviz path not configured (set tools.jviz_path in config.toml or --jviz flag)")
	}
	if _, err := os.Stat(j.binPath); err != nil {
		return fmt.Errorf("jviz binary not found: %s", j.binPath)
	}

	tmpFile, err := os.CreateTemp("", "gem-query-jviz-*.json")
	if err != nil {
		return fmt.Errorf("create temp file: %w", err)
	}
	// Write empty array so jviz can start
	tmpFile.WriteString("[]\n")
	tmpFile.Close()

	j.tmpFile = tmpFile.Name()
	j.port = port

	args := []string{"--watch", j.tmpFile}
	if port != "" {
		args = append(args, "--port", port)
	}

	cmd := exec.Command(j.binPath, args...)
	cmd.Stderr = os.Stderr
	if err := cmd.Start(); err != nil {
		os.Remove(j.tmpFile)
		return fmt.Errorf("start jviz: %w", err)
	}

	j.proc = cmd.Process
	j.active = true
	return nil
}

// update writes a query result to the watched file so jviz refreshes.
func (j *jvizState) update(r *query.Result) error {
	if !j.active || j.tmpFile == "" {
		return nil
	}

	var buf strings.Builder
	if err := output.FormatJSON(&buf, r); err != nil {
		return err
	}
	return os.WriteFile(j.tmpFile, []byte(buf.String()), 0o644)
}

// stop terminates jviz and cleans up.
func (j *jvizState) stop() {
	if j.proc != nil {
		j.proc.Kill()
		j.proc.Wait()
		j.proc = nil
	}
	if j.tmpFile != "" {
		os.Remove(j.tmpFile)
		j.tmpFile = ""
	}
	j.active = false
}
