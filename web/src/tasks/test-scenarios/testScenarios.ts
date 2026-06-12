import type { Priority, TaskType } from "@/types";

/**
 * Difficulty buckets ordered by expected agent effort. Used to group
 * scenarios in the picker UI and gives the operator an at-a-glance signal
 * for "is this a 2-minute warm-up or a 2-hour audit?".
 */
export type TestScenarioDifficulty =
  | "trivial"
  | "easy"
  | "medium"
  | "hard"
  | "expert";

export const TEST_SCENARIO_DIFFICULTY_ORDER: TestScenarioDifficulty[] = [
  "trivial",
  "easy",
  "medium",
  "hard",
  "expert",
];

export const TEST_SCENARIO_DIFFICULTY_LABEL: Record<
  TestScenarioDifficulty,
  string
> = {
  trivial: "Trivial",
  easy: "Easy",
  medium: "Medium",
  hard: "Hard",
  expert: "Expert",
};

/**
 * Operator-facing one-liner that hints at expected agent runtime per
 * difficulty bucket. These are guesses — actual durations vary wildly by
 * codebase size and runner — but they help the operator pick a bucket
 * matching the test they want to run.
 */
export const TEST_SCENARIO_DIFFICULTY_HINT: Record<
  TestScenarioDifficulty,
  string
> = {
  trivial: "≈ a few minutes — read-only or single-file output.",
  easy: "≈ 10–30 minutes — small documentation or mechanical changes.",
  medium: "≈ 30–60 minutes — refactor or validation work with tests.",
  hard: "≈ 1–2 hours — error handling, observability, multi-file changes.",
  expert: "≈ 2+ hours — architecture-shifting refactor or new instrumentation.",
};

/**
 * One ready-to-run test scenario. Picking it auto-fills every form field in
 * the create-task modal so the operator can dispatch a real agent run with
 * zero typing — the whole point of the "test scenarios" affordance.
 *
 * Scenarios are intentionally codebase-agnostic: they reference well-known
 * code constructs ("the longest function", "the README", "the most-called
 * handler") rather than any specific path or symbol. A capable runner
 * (Cursor CLI / Claude Code / Codex) can resolve those references against
 * any repository.
 */
export type TestScenario = {
  id: string;
  difficulty: TestScenarioDifficulty;
  /** Short title shown in the picker; becomes the task `title`. */
  title: string;
  /** One-line description shown beneath the title in the picker. */
  description: string;
  /**
   * Plain-text body inserted into `initial_prompt`. The picker wraps this
   * in `<p>` blocks via `plainTextToInitialHtml` so the rich editor renders
   * paragraphs instead of one mega-line.
   */
  prompt: string;
  /** Best-fit production task type — the operator can override before submit. */
  taskType: TaskType;
  /** Default priority for the auto-fill — almost always "medium". */
  priority: Priority;
  /** Done criteria written into the form's checklist on apply. */
  checklist: string[];
  /**
   * When set, toggles “Start subtasks after parent completes” on the create
   * modal. Omit to leave the operator's current toggle unchanged.
   */
  subtasksWaitForParent?: boolean;
  /**
   * Pending subtasks to pre-fill on apply. Omit to leave any existing pending
   * subtasks unchanged. When present, replaces the whole pending-subtask list.
   */
  pendingSubtasks?: TestScenarioPendingSubtask[];
};

/** One pending subtask row in a scheduling-focused test scenario. */
export type TestScenarioPendingSubtask = {
  title: string;
  prompt?: string;
  priority?: Priority;
  taskType?: TaskType;
  checklist?: string[];
  checklistInherit?: boolean;
  /** Draft-local indices of other pending subtasks in the same scenario. */
  dependsOnSiblingIndices?: number[];
};

/**
 * Curated catalog. Each entry is intentionally small (one objective, a
 * tight prompt, ≤4 done-criteria items) so the operator can read the whole
 * scenario in the popover without scrolling, and the agent has a crisp
 * acceptance bar.
 *
 * Generic across languages and stacks: every scenario uses constructs that
 * exist in any sizable codebase (functions, READMEs, configuration files,
 * hot paths, public APIs). When a scenario says "function" it means
 * function / method / procedure depending on the language.
 */
export const TEST_SCENARIOS: TestScenario[] = [
  // -------- Trivial --------
  {
    id: "trivial.codebase-tour",
    difficulty: "trivial",
    title: "Write a one-paragraph codebase tour",
    description:
      "Skim the top-level config and README, then summarize what the codebase does and how to run it.",
    prompt: [
      "Read the repository's top-level configuration files (e.g. package.json, go.mod, Cargo.toml, pyproject.toml, build.gradle, Makefile) and the README.",
      "Write a one-paragraph summary that answers: what does this codebase do, what is the primary language and runtime, and what is the main entry point?",
      "Save the summary as a markdown file at the repository root named CODEBASE_TOUR.md. Do not modify any other files.",
    ].join("\n\n"),
    taskType: "docs",
    priority: "medium",
    checklist: [
      "CODEBASE_TOUR.md exists at the repository root with the summary.",
      "No other files were modified.",
      "The summary names the primary language, runtime, and main entry point.",
    ],
  },
  {
    id: "trivial.todo-report",
    difficulty: "trivial",
    title: "Catalog every TODO/FIXME/HACK comment",
    description:
      "Search the repo for stale-marker comments and emit a grouped markdown report.",
    prompt: [
      "Search the entire repository for `TODO`, `FIXME`, `HACK`, and `XXX` comments. Exclude vendored / generated code and node_modules-style directories.",
      "Produce a markdown report grouped by file with one bullet per occurrence. Each bullet shows the line number and a one-line summary of the comment in your own words.",
      "Save the report at the repository root as TODO_REPORT.md. Do not modify any source files.",
    ].join("\n\n"),
    taskType: "docs",
    priority: "low",
    checklist: [
      "TODO_REPORT.md exists at the repository root.",
      "Vendored / generated code is excluded.",
      "Each entry has a file path, a line number, and a summary.",
    ],
  },

  // -------- Trivial — T2A subtask scheduling probes --------
  {
    id: "trivial.subtask-parent-gate",
    difficulty: "trivial",
    title: "Probe: parent gates subtasks",
    description:
      "Parent plus two subtasks with “wait for parent” enabled — subtasks should not run until the parent is done.",
    prompt: [
      "This is a T2A scheduling probe, not a codebase audit.",
      "Complete this parent task as quickly as possible: create or update a file at `.agent/subtask-probes/parent-gate-parent.txt` containing a single line `parent done`.",
      "After submit, watch the task list / detail page: the two subtasks must stay blocked (not picked up by the worker) until this parent task reaches status `done`.",
    ].join("\n\n"),
    taskType: "general",
    priority: "medium",
    subtasksWaitForParent: true,
    pendingSubtasks: [
      {
        title: "Subtask A (waits on parent)",
        prompt:
          "Scheduling probe: write `.agent/subtask-probes/parent-gate-a.txt` with one line `subtask a done`. You should only run after the parent probe is `done`.",
        priority: "medium",
        taskType: "general",
        checklist: ["parent-gate-a.txt exists with the expected line."],
      },
      {
        title: "Subtask B (waits on parent)",
        prompt:
          "Scheduling probe: write `.agent/subtask-probes/parent-gate-b.txt` with one line `subtask b done`. You should only run after the parent probe is `done`.",
        priority: "medium",
        taskType: "general",
        checklist: ["parent-gate-b.txt exists with the expected line."],
      },
    ],
    checklist: [
      "Parent probe file exists under `.agent/subtask-probes/`.",
      "Both subtasks were created with depends_on including the parent id.",
    ],
  },
  {
    id: "trivial.subtask-sibling-chain",
    difficulty: "trivial",
    title: "Probe: sibling dependency chain",
    description:
      "Three subtasks in a linear A → B → C chain — each waits for the previous sibling to finish.",
    prompt: [
      "This is a T2A scheduling probe. The parent completes immediately; ordering is entirely between subtasks.",
      "Create `.agent/subtask-probes/sibling-chain-parent.txt` with one line `parent done` and mark this parent task complete.",
      "Do not configure subtasks to wait on the parent — only the sibling chain matters here.",
    ].join("\n\n"),
    taskType: "general",
    priority: "medium",
    subtasksWaitForParent: false,
    pendingSubtasks: [
      {
        title: "Chain step 1",
        prompt:
          "Write `.agent/subtask-probes/sibling-chain-1.txt` with `step 1 done`. No sibling predecessors.",
        priority: "medium",
        taskType: "general",
        checklist: ["sibling-chain-1.txt exists."],
      },
      {
        title: "Chain step 2",
        prompt:
          "Write `.agent/subtask-probes/sibling-chain-2.txt` with `step 2 done`. Run only after step 1 is `done`.",
        priority: "medium",
        taskType: "general",
        dependsOnSiblingIndices: [0],
        checklist: ["sibling-chain-2.txt exists.", "Step 2 ran after step 1."],
      },
      {
        title: "Chain step 3",
        prompt:
          "Write `.agent/subtask-probes/sibling-chain-3.txt` with `step 3 done`. Run only after step 2 is `done`.",
        priority: "medium",
        taskType: "general",
        dependsOnSiblingIndices: [1],
        checklist: ["sibling-chain-3.txt exists.", "Step 3 ran after step 2."],
      },
    ],
    checklist: [
      "Subtasks were created in order with sibling depends_on edges 1→0 and 2→1.",
      "Worker ran step 1, then step 2, then step 3 — never out of order.",
    ],
  },
  {
    id: "trivial.subtask-parent-and-sibling",
    difficulty: "trivial",
    title: "Probe: parent gate + sibling gate",
    description:
      "Batch parent wait plus subtask B waiting on subtask A — B needs both gates cleared.",
    prompt: [
      "T2A combined scheduling probe.",
      "Create `.agent/subtask-probes/combo-parent.txt` with `parent done`. Subtasks wait for this parent AND subtask B also waits on subtask A.",
      "Verify subtask B is the last to run.",
    ].join("\n\n"),
    taskType: "general",
    priority: "medium",
    subtasksWaitForParent: true,
    pendingSubtasks: [
      {
        title: "Combo subtask A",
        prompt:
          "Write `.agent/subtask-probes/combo-a.txt` with `combo a done`. Waits on parent only.",
        priority: "medium",
        taskType: "general",
        checklist: ["combo-a.txt exists."],
      },
      {
        title: "Combo subtask B",
        prompt:
          "Write `.agent/subtask-probes/combo-b.txt` with `combo b done`. Waits on parent AND subtask A.",
        priority: "medium",
        taskType: "general",
        dependsOnSiblingIndices: [0],
        checklist: [
          "combo-b.txt exists.",
          "B ran only after parent and A were both `done`.",
        ],
      },
    ],
    checklist: [
      "Batch “wait for parent” was enabled on create.",
      "Subtask B depends_on includes both parent id and subtask A id.",
      "Worker order was: parent → A → B.",
    ],
  },
  {
    id: "trivial.subtask-parallel-baseline",
    difficulty: "trivial",
    title: "Probe: parallel subtasks (no deps)",
    description:
      "Control case — two independent subtasks with no scheduling gates; worker FIFO among eligible tasks.",
    prompt: [
      "T2A baseline probe: no parent or sibling scheduling gates.",
      "Create `.agent/subtask-probes/parallel-parent.txt` with `parent done`.",
      "Both subtasks should be eligible as soon as they are created (subject to single-worker FIFO).",
    ].join("\n\n"),
    taskType: "general",
    priority: "medium",
    subtasksWaitForParent: false,
    pendingSubtasks: [
      {
        title: "Parallel subtask 1",
        prompt:
          "Write `.agent/subtask-probes/parallel-1.txt` with `parallel 1 done`.",
        priority: "medium",
        taskType: "general",
        checklist: ["parallel-1.txt exists."],
      },
      {
        title: "Parallel subtask 2",
        prompt:
          "Write `.agent/subtask-probes/parallel-2.txt` with `parallel 2 done`.",
        priority: "medium",
        taskType: "general",
        checklist: ["parallel-2.txt exists."],
      },
    ],
    checklist: [
      "Neither subtask has depends_on edges to parent or siblings.",
      "Both subtasks became worker-eligible without waiting on the parent.",
    ],
  },

  // -------- Easy --------
  {
    id: "easy.docstrings-largest-file",
    difficulty: "easy",
    title: "Add docstrings to the largest source file",
    description:
      "Find the biggest source file (excluding tests / generated) and document every public function in it.",
    prompt: [
      "Identify the largest source file in the repository by line count, excluding test files, generated code, vendored dependencies, and minified bundles.",
      "For every public/exported function, method, class, or type in that file that has no docstring or doc comment, add a concise paragraph that says: what it does, what arguments it takes, and what it returns or yields.",
      "Do not change any function bodies or signatures. Match the docstring/comment style already used elsewhere in the codebase. If the language has no convention, use the most idiomatic form (e.g. JSDoc for TypeScript, godoc comments for Go, docstrings for Python).",
    ].join("\n\n"),
    taskType: "docs",
    priority: "medium",
    checklist: [
      "The chosen file is named in the task report.",
      "Every previously undocumented public symbol now has a doc comment.",
      "No function body or signature was modified.",
      "Existing tests still pass.",
    ],
  },
  {
    id: "easy.readme-quickstart",
    difficulty: "easy",
    title: "Add a Repository tour + Quick start to the README",
    description:
      "Audit the README for missing setup sections and fill them in based on actual repo structure.",
    prompt: [
      "Open the project's README.",
      "Add a `## Repository tour` section listing the top-level directories with a one-line description of each (read each directory briefly to write an accurate description).",
      "If the README has no `## Quick start` section, add one with the exact minimum commands needed to install dependencies and run the project locally, derived from the actual config files in the repo.",
      "Do not duplicate sections that already exist; merge intelligently if a similar section is already present.",
    ].join("\n\n"),
    taskType: "docs",
    priority: "medium",
    checklist: [
      "README has a `Repository tour` section listing real top-level directories.",
      "README has a `Quick start` section with verified install + run commands.",
      "No pre-existing README content was deleted or contradicted.",
    ],
  },

  // -------- Medium --------
  {
    id: "medium.split-longest-function",
    difficulty: "medium",
    title: "Split the longest function into focused helpers",
    description:
      "Find the longest function in the repo and refactor it into smaller named helpers without changing behavior.",
    prompt: [
      "Identify the longest function (or method) in the repository by line count, excluding test files and generated code.",
      "If it exceeds 80 lines of executable code (excluding comments and blank lines), refactor it: extract logically grouped chunks into smaller named helper functions in the same file.",
      "Preserve the public signature and observable behavior exactly. Do not rename the original function or change its return type.",
      "If the original function is covered by tests, run them and confirm they still pass. If not, add at least one test that exercises the happy path of the refactored function.",
    ].join("\n\n"),
    taskType: "refactor",
    priority: "medium",
    checklist: [
      "The chosen function is named in the task report along with its original line count.",
      "Helper functions are named descriptively and live in the same file.",
      "The original function's signature and behavior are unchanged.",
      "Pre-existing tests pass; if none existed, at least one new happy-path test was added.",
    ],
  },
  {
    id: "medium.input-validation",
    difficulty: "medium",
    title: "Add input validation to a user-facing function",
    description:
      "Pick a public entry point and harden it against empty / negative / oversized inputs with tests.",
    prompt: [
      "Pick a user-facing entry point in the codebase (HTTP handler, CLI command, public API method, or library export). Prefer one that currently has no input validation.",
      "Add validation that rejects with a clear, structured error for: empty inputs, negative numbers where a positive value is expected, and strings exceeding a reasonable length limit (justify the limit you pick).",
      "Add at least one unit test per new validation branch, asserting both the rejection and the error message shape.",
      "Document the new validation rules in the function's doc comment.",
    ].join("\n\n"),
    taskType: "feature",
    priority: "medium",
    checklist: [
      "The chosen entry point is named with a justification for why it was picked.",
      "Each new validation branch has at least one test.",
      "The function's doc comment lists the new validation rules.",
      "The full test suite still passes.",
    ],
  },

  // -------- Hard --------
  {
    id: "hard.error-handling-hot-path",
    difficulty: "hard",
    title: "Audit and harden error handling on a hot code path",
    description:
      "Find the busiest end-to-end flow and add explicit error handling + tests for every failure branch.",
    prompt: [
      "Identify the hottest code path in the repository — the highest call frequency, the central business logic, or the most-trafficked HTTP/RPC endpoint. Justify the pick.",
      "Audit it for unhandled failure cases: nil/undefined returns, network errors, parse errors, file-not-found, lock contention, unexpected sentinel responses.",
      "Add explicit error handling at the boundary with structured logging that includes enough context for an operator to diagnose the failure. Do not swallow errors silently anywhere.",
      "Add tests that exercise each new error branch (mock or fake the underlying dependency where needed).",
      "Document the new failure-handling contract in the function's doc comment or in the relevant operator-facing doc.",
    ].join("\n\n"),
    taskType: "bug_fix",
    priority: "high",
    checklist: [
      "The chosen path is named with a one-paragraph justification.",
      "Every newly handled failure branch has logging and a test.",
      "No new silent error swallowing was introduced.",
      "The doc comment / operator doc was updated to reflect the new contract.",
    ],
  },
  {
    id: "hard.observability",
    difficulty: "hard",
    title: "Add structured observability to a critical flow",
    description:
      "Pick an end-to-end flow and add structured logs, a metric, and correlation ID propagation.",
    prompt: [
      "Pick a critical end-to-end flow in the codebase — a request lifecycle, a CLI run, a worker job. Justify the pick in one sentence.",
      "Add structured logging at each meaningful state transition: start, every key decision point, terminal states (success / failure). Use the existing logging library and conventions if any; otherwise pick the most idiomatic one for the language.",
      "Add a counter (or equivalent) that records success and failure counts for the flow. Wire it through whatever metrics surface the project already uses; if none exists, expose it as a JSON endpoint or a stdout periodic dump.",
      "Propagate a request / correlation ID through every log line in the flow, generating one at the entry point if no upstream ID is present.",
      "Document the new instrumentation in the project's observability doc (or create OBSERVABILITY.md if none exists).",
    ].join("\n\n"),
    taskType: "feature",
    priority: "high",
    checklist: [
      "The chosen flow is named with a one-line justification.",
      "Every meaningful state transition logs structured fields including the correlation ID.",
      "A success / failure counter (or equivalent) exists and is documented.",
      "Operator-facing observability documentation was updated or created.",
    ],
  },

  // -------- Expert --------
  {
    id: "expert.refactor-for-testability",
    difficulty: "expert",
    title: "Refactor a tightly-coupled module for testability",
    description:
      "Replace concrete I/O calls in a module with a small interface that can be substituted in tests.",
    prompt: [
      "Find a module in the codebase that imports concrete I/O directly (file system, network client, database, child process). It should be one that has poor test coverage today, and the I/O coupling is the reason.",
      "Refactor so the I/O calls go through a small interface (or function-type parameter) that can be substituted in tests. Default production callers wire the real implementation; nothing about observable behavior changes.",
      "Add at least one test that exercises the refactored module against an in-memory implementation of the new interface, covering the happy path and one error path.",
      "Document the new seam in the module's doc comment, including a one-line note on how to inject a fake in tests.",
    ].join("\n\n"),
    taskType: "refactor",
    priority: "high",
    checklist: [
      "The chosen module is named with a one-paragraph justification of the coupling.",
      "Production behavior is unchanged (no public signatures broken).",
      "At least one happy-path and one error-path test exercise the new seam.",
      "The module's doc comment describes how to inject a fake in tests.",
    ],
  },
  {
    id: "expert.backwards-compatible-extension",
    difficulty: "expert",
    title: "Add a backwards-compatible extension to a public API",
    description:
      "Pick a public function or endpoint and add an optional parameter / field that improves it without breaking existing callers.",
    prompt: [
      "Pick a public function or HTTP/RPC endpoint in the codebase that would benefit from a small enhancement (e.g. an optional pagination cursor, an optional `include` flag, an optional response field). Justify the pick.",
      "Implement the enhancement so that all existing callers without the new parameter or field behave identically — no breaking changes anywhere.",
      "Add tests covering: the existing caller behavior (regression), the new feature on the happy path, and at least one error / edge case for the new feature.",
      "Update the API docs / endpoint documentation describing the new parameter or field, including the default value and a usage example.",
    ].join("\n\n"),
    taskType: "feature",
    priority: "high",
    checklist: [
      "The chosen API is named with a justification for the enhancement.",
      "Existing callers' behavior is verified unchanged via a regression test.",
      "The new feature has a happy-path test and at least one edge-case test.",
      "API documentation lists the new parameter / field with a usage example.",
    ],
  },
];

/**
 * Group scenarios by difficulty, preserving the catalog order within each
 * bucket. The picker UI iterates `TEST_SCENARIO_DIFFICULTY_ORDER` and reads
 * each bucket's scenarios from this map so the rendered groups are stable.
 */
export function groupTestScenariosByDifficulty(): Record<
  TestScenarioDifficulty,
  TestScenario[]
> {
  const empty = (): Record<TestScenarioDifficulty, TestScenario[]> => ({
    trivial: [],
    easy: [],
    medium: [],
    hard: [],
    expert: [],
  });
  const byDifficulty = empty();
  for (const scenario of TEST_SCENARIOS) {
    byDifficulty[scenario.difficulty].push(scenario);
  }
  return byDifficulty;
}

export function findTestScenarioById(id: string): TestScenario | undefined {
  return TEST_SCENARIOS.find((scenario) => scenario.id === id);
}
