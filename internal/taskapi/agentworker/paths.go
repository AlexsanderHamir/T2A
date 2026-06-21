package agentworker

import (
	"errors"
	"fmt"
	"log/slog"
)

func assertWorkingDirExists(dir string) error {
	slog.Debug("trace", "cmd", logCmd, "operation", "taskapi.assertWorkingDirExists",
		"dir", dir)
	if dir == "" {
		return errors.New("working directory is empty")
	}
	info, err := pathProbe.Stat(dir)
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
	if err := pathProbe.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("mkdir %q: %w", dir, err)
	}
	probePath, err := pathProbe.CreateTemp(dir, ".hamix-worker-probe-*")
	if err != nil {
		return fmt.Errorf("write probe in %q: %w", dir, err)
	}
	_ = pathProbe.Remove(probePath)
	return nil
}
