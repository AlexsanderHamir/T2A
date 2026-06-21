package agentworker

import (
	"errors"
	"io/fs"
	"os"
	"testing"
	"time"
)

type fakeFileInfo struct {
	dir bool
}

func (f fakeFileInfo) Name() string       { return "fake" }
func (f fakeFileInfo) Size() int64        { return 0 }
func (f fakeFileInfo) Mode() fs.FileMode  { return 0 }
func (f fakeFileInfo) ModTime() time.Time { return time.Time{} }
func (f fakeFileInfo) IsDir() bool        { return f.dir }
func (f fakeFileInfo) Sys() any           { return nil }

type memPathProbeFS struct {
	statResult fakeFileInfo
	statErr    error
	mkdirErr   error
	tempPath   string
	tempErr    error
}

func (m *memPathProbeFS) Stat(name string) (os.FileInfo, error) {
	if m.statErr != nil {
		return nil, m.statErr
	}
	return m.statResult, nil
}

func (m *memPathProbeFS) MkdirAll(path string, perm os.FileMode) error {
	return m.mkdirErr
}

func (m *memPathProbeFS) CreateTemp(dir, pattern string) (string, error) {
	if m.tempErr != nil {
		return "", m.tempErr
	}
	if m.tempPath != "" {
		return m.tempPath, nil
	}
	return dir + "/.hamix-worker-probe-test", nil
}

func (m *memPathProbeFS) Remove(path string) error {
	return nil
}

func withPathProbeFSForTest(t *testing.T, fs pathProbeFS) {
	t.Helper()
	prev := pathProbe
	pathProbe = fs
	t.Cleanup(func() { pathProbe = prev })
}

func TestAssertWorkingDirExists_happyPath(t *testing.T) {
	withPathProbeFSForTest(t, &memPathProbeFS{
		statResult: fakeFileInfo{dir: true},
	})
	if err := assertWorkingDirExists("/repo"); err != nil {
		t.Fatalf("assertWorkingDirExists: %v", err)
	}
}

func TestAssertWorkingDirExists_statError(t *testing.T) {
	statErr := errors.New("permission denied")
	withPathProbeFSForTest(t, &memPathProbeFS{statErr: statErr})
	err := assertWorkingDirExists("/missing")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, statErr) {
		t.Fatalf("expected wrapped stat error, got %v", err)
	}
}

func TestEnsureWorkerReportDirWritable_happyPath(t *testing.T) {
	withPathProbeFSForTest(t, &memPathProbeFS{})
	if err := ensureWorkerReportDirWritable("/reports"); err != nil {
		t.Fatalf("ensureWorkerReportDirWritable: %v", err)
	}
}

func TestEnsureWorkerReportDirWritable_createTempError(t *testing.T) {
	tempErr := errors.New("read-only filesystem")
	withPathProbeFSForTest(t, &memPathProbeFS{tempErr: tempErr})
	err := ensureWorkerReportDirWritable("/reports")
	if err == nil {
		t.Fatal("expected error")
	}
	if !errors.Is(err, tempErr) {
		t.Fatalf("expected wrapped temp error, got %v", err)
	}
}
