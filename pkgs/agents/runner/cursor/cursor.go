package cursor

import (
	"bytes"
	"context"
	"fmt"
	"log/slog"
	"strings"

	"github.com/AlexsanderHamir/T2A/pkgs/agents/runner"
	"github.com/AlexsanderHamir/T2A/pkgs/tasks/domain"
)

// Options configures an Adapter at construction time.
type Options struct {
	// BinaryPath is the cursor-agent executable. Defaults to "cursor-agent"
	// (resolved against PATH).
	BinaryPath string
	// Args is an optional fixed argv tail (tests). When nil, argv is built
	// per Run from DefaultCursorModel and runner.Request.CursorModel.
	// When non-nil, Args is passed verbatim and DefaultCursorModel /
	// Request.CursorModel are ignored.
	Args []string
	// DefaultCursorModel is used when Request.CursorModel is empty (typical
	// app-settings default at worker construction).
	DefaultCursorModel string
	// Name is the runner.Name() value (recorded in TaskCyclePhase MetaJSON
	// by the worker). Defaults to "cursor-cli".
	Name string
	// Version is the runner.Version() value. Defaults to
	// "0.0.0-unknown"; binaries should override with the real value.
	Version string
	// ExecFn replaces os/exec for tests. nil means use the real exec path.
	ExecFn ExecFn
	// StreamExecFn replaces the live os/exec path for tests that need
	// incremental stdout delivery. When both ExecFn and StreamExecFn are
	// nil, the adapter uses the real streaming exec path. ExecFn takes
	// precedence to keep existing batch tests stable.
	StreamExecFn StreamExecFn
	// ExtraAllowedEnvKeys widens the parent-env passthrough beyond the
	// default curated allowlist. Entries are still subject to the deny-list
	// (DATABASE_URL, T2A_*).
	ExtraAllowedEnvKeys []string
	// HomePathReplacements lets tests inject the values used to scrub
	// absolute home paths from RawOutput. Empty slice means use the live
	// $HOME and $USERPROFILE.
	HomePathReplacements []string
}

// Adapter is the cursor.Runner implementation. Construct via New.
type Adapter struct {
	binaryPath         string
	args               []string
	defaultCursorModel string
	name               string
	version            string
	exec               ExecFn
	streamExec         StreamExecFn
	extraKeys          []string
	homePaths          []string
}

// New returns a configured Adapter. Zero-value Options yields the documented
// defaults.
func New(opts Options) *Adapter {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.New")
	a := &Adapter{
		binaryPath:         opts.BinaryPath,
		defaultCursorModel: strings.TrimSpace(opts.DefaultCursorModel),
		name:               opts.Name,
		version:            opts.Version,
		exec:               opts.ExecFn,
		streamExec:         opts.StreamExecFn,
		extraKeys:          append([]string(nil), opts.ExtraAllowedEnvKeys...),
		homePaths:          append([]string(nil), opts.HomePathReplacements...),
	}
	if len(opts.Args) > 0 {
		a.args = append([]string(nil), opts.Args...)
	}
	if a.binaryPath == "" {
		a.binaryPath = defaults.BinaryPath
	}
	if a.name == "" {
		a.name = defaults.Name
	}
	if a.version == "" {
		a.version = defaults.Version
	}
	if a.exec == nil && a.streamExec == nil {
		a.streamExec = defaultStreamExecFn
	}
	if len(a.homePaths) == 0 {
		a.homePaths = liveHomePaths()
	}
	return a
}

func (a *Adapter) argvFor(req runner.Request) []string {
	if len(a.args) > 0 {
		return a.args
	}
	m := strings.TrimSpace(req.CursorModel)
	if m == "" {
		m = a.defaultCursorModel
	}
	out := []string{cursorFlagPrint, cursorFlagOutputFormat, cursorOutputFormatStreamJSON}
	if m != "" {
		out = append(out, cursorFlagModel, m)
	}
	out = append(out, cursorFlagForce)
	return out
}

// Name implements runner.Runner.
func (a *Adapter) Name() string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.Adapter.Name")
	return a.name
}

// Version implements runner.Runner.
func (a *Adapter) Version() string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.Adapter.Version")
	return a.version
}

// EffectiveModel implements runner.Runner. It mirrors the fallback applied
// inside argvFor.
func (a *Adapter) EffectiveModel(req runner.Request) string {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.Adapter.EffectiveModel",
		"task_id", req.TaskID)
	m := strings.TrimSpace(req.CursorModel)
	if m != "" {
		return m
	}
	return a.defaultCursorModel
}

// Run implements runner.Runner. See package documentation for the full
// invocation contract, env policy, redaction guarantees, and error mapping.
func (a *Adapter) Run(ctx context.Context, req runner.Request) (runner.Result, error) {
	slog.Debug("trace", "cmd", cursorLogCmd, "operation", "cursor.Adapter.Run",
		"task_id", req.TaskID, "phase", string(req.Phase),
		"attempt_seq", req.AttemptSeq, "working_dir", req.WorkingDir,
		"timeout_ns", int64(req.Timeout))

	if err := ctx.Err(); err != nil {
		return runner.Result{}, fmt.Errorf("cursor: %w: %v", runner.ErrTimeout, err)
	}

	runCtx, cancel := withOptionalTimeout(ctx, req.Timeout)
	defer cancel()

	env := buildEnv(req.Env, a.extraKeys)
	argv := a.argvFor(req)
	var stdout, stderr []byte
	var exitCode int
	var execErr error
	if a.streamExec != nil {
		stdout, stderr, exitCode, execErr = a.streamExec(
			runCtx,
			req.WorkingDir,
			env,
			[]byte(req.Prompt),
			a.binaryPath,
			func(line []byte) {
				emitProgressFromLine(req.OnProgress, line, a.homePaths)
			},
			argv...,
		)
	} else {
		stdout, stderr, exitCode, execErr = a.exec(
			runCtx,
			req.WorkingDir,
			env,
			[]byte(req.Prompt),
			a.binaryPath,
			argv...,
		)
	}

	rawOutput := redact(combineStreams(stdout, stderr), a.homePaths)

	if execErr != nil && !isCtxErr(runCtx) && isClosedPipeReadError(execErr) && len(bytes.TrimSpace(stdout)) > 0 {
		slog.Debug("ignoring closed stdout pipe after cursor output",
			"cmd", cursorLogCmd, "operation", "cursor.Adapter.Run.closed_pipe_with_stdout",
			"stdout_bytes", len(stdout), "stderr_bytes", len(stderr), "err", execErr)
		execErr = nil
	}

	if execErr != nil {
		if isCtxErr(runCtx) {
			return runner.NewResult(domain.PhaseStatusFailed, timeoutSummary(req.Timeout),
					failureDetails("timeout", execErr, stdout, stderr, a.homePaths, map[string]any{
						"timeout_ns":         int64(req.Timeout),
						"timeout_configured": req.Timeout > 0,
					}), rawOutput),
				fmt.Errorf("cursor: %w: %v", runner.ErrTimeout, execErr)
		}
		return runner.NewResult(domain.PhaseStatusFailed, execFailedSummary(execErr, a.homePaths),
				failureDetails("exec", execErr, stdout, stderr, a.homePaths, map[string]any{
					"binary":      redact(a.binaryPath, a.homePaths),
					"argv":        argv,
					"working_dir": redact(req.WorkingDir, a.homePaths),
				}), rawOutput),
			fmt.Errorf("cursor: %w: %v", runner.ErrInvalidOutput, execErr)
	}

	if exitCode != 0 {
		details := stderrTailDetails(stderr, a.homePaths)
		combined := string(stderr) + "\n" + string(stdout)
		kind, stdMsg := classifyCursorFailure(combined)
		if kind != "" {
			details = mergeDetailsJSON(details, map[string]any{
				"failure_kind":         kind,
				"standardized_message": stdMsg,
			})
		}
		summary := fmt.Sprintf("cursor: exit %d", exitCode)
		switch kind {
		case FailureKindCursorUsageLimit:
			summary = titleForFailureKind(kind)
		default:
			if hint := stderrFirstLineHint(stderr, a.homePaths); hint != "" {
				summary = summary + ": " + hint
			}
		}
		return runner.NewResult(domain.PhaseStatusFailed, summary, details, rawOutput),
			fmt.Errorf("cursor: %w: exit %d", runner.ErrNonZeroExit, exitCode)
	}

	parsed, parseErr := parseStdout(stdout)
	if parseErr != nil {
		return runner.NewResult(domain.PhaseStatusFailed, invalidOutputSummary(parseErr, a.homePaths),
				failureDetails("parse_stdout", parseErr, stdout, stderr, a.homePaths, nil), rawOutput),
			fmt.Errorf("cursor: %w: %v", runner.ErrInvalidOutput, parseErr)
	}

	summary := redact(parsed.Result, a.homePaths)
	details := buildDetails(parsed)

	if parsed.IsError {
		if summary == "" {
			summary = "cursor: agent reported is_error=true"
		}
		res := runner.NewResult(domain.PhaseStatusFailed, summary, details, rawOutput)
		res.ResolvedModel = parsed.ResolvedModel
		return res, fmt.Errorf("cursor: %w: agent reported is_error=true", runner.ErrNonZeroExit)
	}

	res := runner.NewResult(domain.PhaseStatusSucceeded, summary, details, rawOutput)
	res.ResolvedModel = parsed.ResolvedModel
	return res, nil
}

// Compile-time assertion that *Adapter implements runner.Runner.
var _ runner.Runner = (*Adapter)(nil)
