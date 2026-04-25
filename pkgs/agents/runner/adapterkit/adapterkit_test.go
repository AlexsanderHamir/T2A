package adapterkit_test

import (
	"bytes"
	"context"
	"errors"
	"os"
	"reflect"
	"strings"
	"testing"
	"time"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner/adapterkit"
)

func TestBuildEnv_deniesConfiguredKeysAndPrefixes(t *testing.T) {
	t.Parallel()

	policy := adapterkit.EnvPolicy{
		ParentAllowedKeys: []string{"PATH", "DATABASE_URL", "T2A_SECRET"},
		ExtraAllowedKeys:  []string{"EXTRA", "T2A_EXTRA"},
		DeniedKeys:        []string{"DATABASE_URL"},
		DeniedPrefixes:    []string{"T2A_"},
		Lookup: func(k string) string {
			return map[string]string{
				"PATH":         "/bin",
				"DATABASE_URL": "postgres://secret",
				"T2A_SECRET":   "secret",
				"EXTRA":        "extra-from-parent",
			}[k]
		},
	}

	got := envSliceToMap(adapterkit.BuildEnv(map[string]string{
		"PATH":         "/custom/bin",
		"DATABASE_URL": "postgres://request-secret",
		"T2A_EXTRA":    "secret",
		"SAFE":         "request-safe",
	}, policy))

	if got["PATH"] != "/custom/bin" {
		t.Fatalf("PATH = %q, want request override", got["PATH"])
	}
	if got["SAFE"] != "request-safe" {
		t.Fatalf("SAFE = %q", got["SAFE"])
	}
	for _, denied := range []string{"DATABASE_URL", "T2A_SECRET", "T2A_EXTRA"} {
		if _, ok := got[denied]; ok {
			t.Fatalf("denied key %q leaked into child env: %#v", denied, got)
		}
	}
}

func TestRedact_scrubsSharedSecretShapes(t *testing.T) {
	t.Parallel()

	got := adapterkit.Redact(
		"Authorization: Bearer abc.def\nCookie: sid=cookie-secret\nSet-Cookie: x=y\nT2A_FOO=secret\n/home/me/repo",
		adapterkit.DefaultRedactionPolicy([]string{"/home/me"}),
	)

	for _, leaked := range []string{"abc.def", "cookie-secret", "x=y", "secret", "/home/me"} {
		if strings.Contains(got, leaked) {
			t.Fatalf("redaction leaked %q in %q", leaked, got)
		}
	}
	if !strings.Contains(got, "Authorization: [REDACTED]") || !strings.Contains(got, "~/repo") {
		t.Fatalf("redaction did not preserve expected markers: %q", got)
	}
}

func TestRedactedTail_keepsUTF8Boundary(t *testing.T) {
	t.Parallel()

	input := []byte("prefix 日本語")
	got := adapterkit.RedactedTail(input, adapterkit.RedactionPolicy{}, len("本語"))

	if !strings.Contains(got, "語") {
		t.Fatalf("tail = %q, want trailing UTF-8 text", got)
	}
	if strings.ContainsRune(got, '\uFFFD') {
		t.Fatalf("tail contains replacement rune: %q", got)
	}
}

func TestScanStdoutLines_copiesLinesAndCapturesOutput(t *testing.T) {
	t.Parallel()

	var dst bytes.Buffer
	var lines [][]byte
	err := adapterkit.ScanStdoutLines(strings.NewReader("one\ntwo\n"), &dst, func(line []byte) {
		lines = append(lines, line)
		line[0] = 'x'
	})
	if err != nil {
		t.Fatalf("ScanStdoutLines: %v", err)
	}
	if got := dst.String(); got != "one\ntwo\n" {
		t.Fatalf("captured stdout = %q", got)
	}
	if got := string(lines[0]); got != "xne" {
		t.Fatalf("callback line mutation should not affect dst but should mutate callback copy, got %q", got)
	}
}

func TestRunProbe_appliesTimeout(t *testing.T) {
	t.Parallel()

	probeErr := errors.New("deadline")
	fake := func(ctx context.Context, name string, args ...string) ([]byte, []byte, int, error) {
		<-ctx.Done()
		return nil, nil, 0, probeErr
	}
	_, _, _, err := adapterkit.RunProbe(context.Background(), 10*time.Millisecond, fake, "runner", "--version")
	if !errors.Is(err, probeErr) {
		t.Fatalf("RunProbe err = %v, want %v", err, probeErr)
	}
}

func TestDefaultExec_nonZeroExitReturnsExitCode(t *testing.T) {
	t.Parallel()

	stdout, stderr, exitCode, err := adapterkit.DefaultExec(
		context.Background(),
		"",
		append(os.Environ(), "ADAPTERKIT_TEST_CHILD=1"),
		nil,
		os.Args[0],
		"-test.run=TestHelperProcess_DefaultExec",
	)
	if err != nil {
		t.Fatalf("DefaultExec err = %v stdout=%s stderr=%s", err, stdout, stderr)
	}
	if exitCode != 7 {
		t.Fatalf("exitCode = %d, want 7", exitCode)
	}
}

func TestHelperProcess_DefaultExec(t *testing.T) {
	if os.Getenv("ADAPTERKIT_TEST_CHILD") != "1" {
		return
	}
	os.Exit(7)
}

func TestCombineStreams_labelsOutput(t *testing.T) {
	t.Parallel()

	got := adapterkit.CombineStreams([]byte("out"), []byte("err"))
	want := "[stdout]\nout\n[stderr]\nerr"
	if got != want {
		t.Fatalf("CombineStreams = %q, want %q", got, want)
	}
}

func TestFirstNonEmptyLineAndTrimForLog(t *testing.T) {
	t.Parallel()

	if got := adapterkit.FirstNonEmptyLine([]byte("\n  version 1\nbuild\n")); got != "version 1" {
		t.Fatalf("FirstNonEmptyLine = %q", got)
	}
	if got := adapterkit.TrimForLog([]byte("abcdef"), 3); got != "abc…" {
		t.Fatalf("TrimForLog = %q", got)
	}
}

func envSliceToMap(env []string) map[string]string {
	out := make(map[string]string, len(env))
	for _, kv := range env {
		parts := strings.SplitN(kv, "=", 2)
		if len(parts) == 2 {
			out[parts[0]] = parts[1]
		}
	}
	return out
}

func TestBuildEnv_returnsNoDuplicateKeys(t *testing.T) {
	t.Parallel()

	env := adapterkit.BuildEnv(map[string]string{"PATH": "/request"}, adapterkit.EnvPolicy{
		ParentAllowedKeys: []string{"PATH"},
		ExtraAllowedKeys:  []string{"PATH"},
		Lookup: func(string) string {
			return "/parent"
		},
	})
	keys := make([]string, 0, len(env))
	for _, kv := range env {
		keys = append(keys, strings.SplitN(kv, "=", 2)[0])
	}
	if !reflect.DeepEqual(keys, []string{"PATH"}) {
		t.Fatalf("keys = %v, want one PATH", keys)
	}
}
