package agentworker

import (
	"errors"
	"fmt"
	"log/slog"
	"os"
)

func assertWorkingDirExists(dir string) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "taskapi.assertWorkingDirExists",
		"dir", dir)
	if dir == "" {
		return errors.New("working directory is empty")
	}
	info, err := os.Stat(dir)
	if err != nil {
		return fmt.Errorf("stat %q: %w", dir, err)
	}
	if !info.IsDir() {
		return fmt.Errorf("path %q is not a directory", dir)
	}
	return nil
}

func ensureWorkerReportDirWritable(dir string) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "taskapi.ensureWorkerReportDirWritable",
		"dir", dir)
	if dir == "" {
		return errors.New("report dir is empty")
	}
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", dir, err)
	}
	probe, err := os.CreateTemp(dir, ".t2a-worker-probe-*")
	if err != nil {
		return fmt.Errorf("write probe in %q: %w", dir, err)
	}
	probePath := probe.Name()
	_ = probe.Close()
	_ = os.Remove(probePath)
	return nil
}
