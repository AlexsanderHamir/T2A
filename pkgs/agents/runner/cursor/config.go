package cursor

import "github.com/AlexsanderHamir/T2A/pkgs/agents/runner/adapterkit"

const cursorLogCmd = "taskapi"

// Defaults groups the Cursor adapter's stable identity and invocation values.
type Defaults struct {
	Name       string
	Version    string
	BinaryPath string
}

// Limits groups byte/rune/time budgets used while adapting Cursor output.
type Limits struct {
	StderrTailBytes        int
	DiagnosticTailBytes    int
	StderrSummaryHintRunes int
	ProgressSummaryRunes   int
}

var (
	defaults = Defaults{
		Name:       "cursor-cli",
		Version:    "0.0.0-unknown",
		BinaryPath: "cursor-agent",
	}
	limits = Limits{
		StderrTailBytes:        8 * 1024,
		DiagnosticTailBytes:    4 * 1024,
		StderrSummaryHintRunes: 280,
		ProgressSummaryRunes:   240,
	}
)

const (
	cursorFlagPrint        = "--print"
	cursorFlagOutputFormat = "--output-format"
	cursorFlagModel        = "--model"
	cursorFlagForce        = "--force"

	cursorOutputFormatStreamJSON = "stream-json"

	cursorEventSystem    = "system"
	cursorEventAssistant = "assistant"
	cursorEventToolCall  = "tool_call"
	cursorEventResult    = "result"

	cursorSubtypeInit      = "init"
	cursorSubtypeStarted   = "started"
	cursorSubtypeStart     = "start"
	cursorSubtypeCompleted = "completed"
	cursorSubtypeSuccess   = "success"
	cursorSubtypeDone      = "done"
	cursorSubtypeFailed    = "failed"
	cursorSubtypeError     = "error"

	cursorContentText = "text"
)

// ExecFn is the seam unit tests use to avoid shelling out. It receives
// everything the adapter would pass to os/exec and returns the captured
// stdout, stderr, exit code, and error. A nil error with a non-zero
// exitCode means the process ran to completion but exited unsuccessfully.
// A non-nil error means the process did not complete (start failure,
// killed by ctx, etc).
type ExecFn = adapterkit.ExecFunc

// StreamExecFn is the production execution path for live cursor-agent
// progress. It invokes onStdoutLine once per complete stdout line while
// the child is still running, then returns the full captured streams so
// Run can build the durable terminal Result exactly as before.
type StreamExecFn = adapterkit.StreamExecFunc

// defaultPassthroughEnvKeys is the curated list of parent-process env
// vars the cursor adapter forwards to every spawned cursor-agent child
// by default. Grouped by purpose:
//
//   - Universal: PATH, HOME, USERPROFILE (binary lookup + user home on
//     both Unix and Windows).
//   - Windows process model and command interpreter: SYSTEMDRIVE,
//     SYSTEMROOT, WINDIR, COMSPEC, PATHEXT.
//   - Windows known folders that ExpandEnvironmentStrings and
//     SHGetKnownFolderPath rely on: LOCALAPPDATA, APPDATA, PROGRAMDATA,
//     ALLUSERSPROFILE, PUBLIC, TEMP, TMP.
//   - Windows program / DLL lookup: PROGRAMFILES, PROGRAMFILES(X86),
//     PROGRAMW6432, COMMONPROGRAMFILES, COMMONPROGRAMFILES(X86).
//   - Windows identity (used by Win32 APIs that resolve principals or
//     compute per-user paths from scratch): USERNAME, USERDOMAIN,
//     COMPUTERNAME, LOGONSERVER, SESSIONNAME.
//   - Architecture / CPU info (build tools and runtime probes): OS,
//     PROCESSOR_ARCHITECTURE, PROCESSOR_IDENTIFIER, PROCESSOR_LEVEL,
//     PROCESSOR_REVISION, NUMBER_OF_PROCESSORS.
//
// Why the Windows system vars are mandatory and not optional: when
// SYSTEMDRIVE was missing from the child env block on Windows, components in
// the cursor-agent process tree called ExpandEnvironmentStrings on hardcoded
// paths like "%SystemDrive%\\ProgramData\\..." against the empty env. Windows
// then treated those tokens as literals and resolved them under RepoRoot,
// silently creating a literal "%SystemDrive%" directory tree in the worktree.
var defaultPassthroughEnvKeys = []string{
	"PATH",
	"HOME",
	"USERPROFILE",
	"SYSTEMDRIVE",
	"SYSTEMROOT",
	"WINDIR",
	"COMSPEC",
	"PATHEXT",
	"LOCALAPPDATA",
	"APPDATA",
	"PROGRAMDATA",
	"ALLUSERSPROFILE",
	"PUBLIC",
	"TEMP",
	"TMP",
	"PROGRAMFILES",
	"PROGRAMFILES(X86)",
	"PROGRAMW6432",
	"COMMONPROGRAMFILES",
	"COMMONPROGRAMFILES(X86)",
	"USERNAME",
	"USERDOMAIN",
	"COMPUTERNAME",
	"LOGONSERVER",
	"SESSIONNAME",
	"OS",
	"PROCESSOR_ARCHITECTURE",
	"PROCESSOR_IDENTIFIER",
	"PROCESSOR_LEVEL",
	"PROCESSOR_REVISION",
	"NUMBER_OF_PROCESSORS",
}

func envPolicy(extraKeys []string) adapterkit.EnvPolicy {
	return adapterkit.EnvPolicy{
		ParentAllowedKeys: defaultPassthroughEnvKeys,
		ExtraAllowedKeys:  extraKeys,
		DeniedKeys:        []string{"DATABASE_URL"},
		DeniedPrefixes:    []string{"T2A_"},
	}
}

func redactionPolicy(homePaths []string) adapterkit.RedactionPolicy {
	return adapterkit.DefaultRedactionPolicy(homePaths)
}
