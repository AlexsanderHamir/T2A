# Pending notes for the operator

A scratch surface where AI agents leave **questions, technical-choice trade-offs, or
follow-ups** that need a human decision but were not blocking enough to halt the
session. Read this **at the start of every new task** — the operator has asked
agents to surface these explicitly so nothing slips while running inlined work.

When you start a session:

1. Read this file. If anything is relevant to the work you're about to do, raise
   it before you make changes that would silently lock in a default.
2. When you finish a session, append any new follow-ups under the matching
   **Open** subsection. Do not delete entries unless the operator has resolved
   them.
3. When the operator confirms a resolution, move the entry to **Resolved** with
   the date and a one-line outcome.

Format for each entry:

```
### Title
- Date: YYYY-MM-DD
- From task: <short label>
- Decision needed: <one sentence>
- Default chosen if no answer: <what the agent did anyway>
- Files affected: <paths>
```

---

## Open

_(no open items — every entry has been resolved.)_

---

## Resolved

### REFERENCES block lives outside the TipTap document
- Date: 2026-04-29
- From task: Project Context Mentions
- Decision needed: Should the read-only REFERENCES block be a non-editable
  TipTap node inside the prompt document, or stay as a sibling React component
  rendered above `<EditorContent>` (current choice)?
- Default chosen if no answer: Sibling React component. Keeps `initial_prompt`
  HTML clean (no extra wrapper tags sneaking into stored prompts), keeps the
  backend prompt-injection contract unchanged, and makes the block trivially
  non-editable by construction.
- Files affected: `web/src/tasks/components/rich-prompt/RichPromptEditor.tsx`,
  `web/src/tasks/components/rich-prompt/ProjectReferencesBlock.tsx`.
- Resolution (2026-04-29): Confirmed sibling-component default. The contract
  with the backend (IDs are the source of truth, prompt HTML stays clean) is
  the right boundary; embedding the block as a TipTap node would invite
  accidental edits and complicate prompt sanitization.

### Single chip per `#` selection regardless of children
- Date: 2026-04-29
- From task: Project Context Mentions
- Decision needed: When the operator picks "Reference this node and its
  children", should the editor insert one chip per node or just one chip for
  the picked node (with the descendants only showing in the REFERENCES block)?
- Default chosen if no answer: One chip for the picked node. Avoids polluting
  the prompt body with a long list of chips when a deep tree is referenced;
  the REFERENCES block always shows the full expanded set.
- Files affected: `web/src/tasks/components/rich-prompt/RichPromptEditor.tsx`
  (`insertProjectContextChip`).
- Resolution (2026-04-29): Confirmed single-chip default. The chip is the
  operator's anchor in the prose; the REFERENCES block is the audit surface.
  Painting one chip per descendant would crowd the prompt and confuse the
  intent — "I referenced one branch" reads more clearly as one chip.

### Short ID format = first 6 alphanumerics, lowercased
- Date: 2026-04-29
- From task: Project Context Mentions
- Decision needed: Is six characters enough to disambiguate context items
  side-by-side, or should we use a longer suffix (8+) for projects with many
  similarly-named nodes?
- Default chosen if no answer: 6 chars. Matches the example in the original
  plan (`#Decision title · a1b2c3`) and reads compactly inside chips.
- Files affected: `web/src/projects/projectContextRefs.ts`
  (`PROJECT_CONTEXT_SHORT_ID_LENGTH`).
- Resolution (2026-04-29): Confirmed 6-char default. Six lowercase
  alphanumerics yield ~2.2 billion unique prefixes — collision is effectively
  impossible at any realistic project size. If a real collision ever surfaces
  inside a single project we can grow `PROJECT_CONTEXT_SHORT_ID_LENGTH` in
  one place; the shape stays stable.

### Choose context chooser still exposes list + tree views
- Date: 2026-04-29
- From task: Project Context Mentions
- Decision needed: Plan said "compact selected-context panel"; the chooser
  modal itself still has the full list/tree toggle for browsing. Is that the
  intended scope, or should the chooser collapse to a single search-driven
  picker now that `#` is the primary add path?
- Default chosen if no answer: Kept the existing list/tree toggle for
  discovery; only changed the selection semantics (clicking a node now opens
  the node-only / with-children dialog, replacing the old multi-checkbox
  flow).
- Files affected: `web/src/projects/ProjectContextPicker.tsx`.
- Resolution (2026-04-29): Confirmed the discovery path stays. `#` is the
  speed path for "I know what I want"; the chooser is the discovery path for
  "I'm browsing what's available". Keeping both honors the two distinct user
  intents instead of forcing every operator into the same flow.

### Quick-pick offset bucket bounds
- Date: 2026-04-29
- From task: Schedule quick-pick popover
- Decision needed: The new offset popover exposes
  Minutes (10..50 step 10), Hours (1..24), Days (1..6), Weeks (1..3),
  Months (1..12). The operator's spec said "List of minutes up to one
  hour" / "List of hours from 1 to 24hrs" / "Same thing for days, weeks,
  months" without explicit bounds for days/weeks/months. Are these ranges
  right, or should we extend (e.g. months 1..24, days 1..14, weeks 1..8)?
- Default chosen if no answer: Bounds chosen so adjacent buckets do not
  overlap (7d → 1w → 1mo). Keeps the popover visually compact while still
  covering common deferrals.
- Files affected: `web/src/shared/time/quickScheduleOffsets.ts`
  (`QUICK_OFFSET_BUCKETS`).
- Resolution (2026-04-29): Confirmed non-overlapping bounds. The mental
  model "minutes → hours → days → weeks → months" only stays clean if each
  bucket stops where the next one starts. Edge cases beyond the bounds (90
  days, 6 months) can still be entered via the date+time inputs above the
  popover, so the popover stays compact without locking the operator out.

### Month arithmetic clamps to last day of target month
- Date: 2026-04-29
- From task: Schedule quick-pick popover
- Decision needed: When the source date does not exist in the target month
  (e.g. Jan 31 + 1 month → Feb 31), should the picker clamp to the last
  day of the target month (chosen) or roll forward to the next month
  (Mar 3)?
- Default chosen if no answer: Clamp to last day of target month. Matches
  how every major calendar app (Apple Calendar, Google Calendar,
  iCal RFC 5545 BYMONTHDAY rules) handles the same edge case, and keeps
  the picked instant inside the month the operator visually expects.
- Files affected: `web/src/shared/time/quickScheduleOffsets.ts`
  (`computeOffsetIso` month branch).
- Resolution (2026-04-29): Confirmed clamping. Industry-standard behavior
  (RFC 5545 BYMONTHDAY clamp); rolling forward would surprise operators
  who picked "+1 month" expecting to land in February, not early March.

### Single trigger label "Schedule for later…"
- Date: 2026-04-29
- From task: Schedule quick-pick popover
- Decision needed: The QUICK PICK row now has a single trigger button
  labelled "Schedule for later…" with a clock icon. Is the copy / icon
  pairing right, or should it be more explicit (e.g. "Pick a delay…",
  "Schedule (10m / 1h / 1d / 1w)")?
- Default chosen if no answer: "Schedule for later…" reads as a
  natural-language affordance and the trailing ellipsis signals "opens
  more UI". The clock glyph mirrors the calendar glyph on the input field
  so the two affordances read as siblings.
- Files affected: `web/src/shared/time/SchedulePicker.tsx`
  (trigger button JSX).
- Resolution (2026-04-29): Confirmed copy + icon pairing. Verb-first
  natural language ("Schedule…") plus the ellipsis-disclosure convention is
  the same affordance vocabulary Stripe / Linear use; trying to enumerate
  examples in the label ("10m / 1h / 1d") would only crowd the button.

### Test scenarios surface as a header trigger, not a new task type
- Date: 2026-04-29
- From task: Pre-defined test scenarios
- Decision needed: The operator asked for "pre-defined task types for
  testing". Should they live as a new `task_type` enum value (alongside
  General / Bug fix / Feature / Refactor / Docs / DMAP), or as a
  separate "Insert a scenario" affordance that auto-fills the existing
  fields without changing the task taxonomy?
- Default chosen if no answer: Separate affordance — a "Test scenarios"
  trigger button in the create-modal header opens a popover that fills
  Title / Prompt / Priority / Task type / Done criteria. Avoids backend
  schema / migration changes, keeps production task-type analytics clean,
  and lets each scenario pick a real production task type
  (`refactor`, `docs`, `bug_fix`, `feature`) that the agent worker
  already understands.
- Files affected: `web/src/tasks/test-scenarios/`,
  `web/src/tasks/components/task-create-modal/TestScenariosTrigger.tsx`,
  `web/src/tasks/components/task-create-modal/TestScenariosPopover.tsx`,
  `web/src/tasks/components/task-create-modal/TaskCreateModal.tsx`,
  `web/src/tasks/hooks/useTaskCreateFlow.ts` (`applyTestScenario`).
- Resolution (2026-04-29): Confirmed header-trigger pattern. Adding a
  `task_type=test` enum value would pollute production analytics (every
  metric would need a "real tasks vs test scenarios" filter forever) and
  require a backend migration for what is fundamentally a UI-only
  affordance. The trigger / popover is a zero-cost, zero-migration path.

### Test scenarios catalog scope (10 entries, 2 per difficulty)
- Date: 2026-04-29
- From task: Pre-defined test scenarios
- Decision needed: The catalog ships with 10 scenarios — 2 per difficulty
  (Trivial / Easy / Medium / Hard / Expert). Should we ship more (e.g.
  3–4 per bucket), or fewer? Should we add language-specific scenarios
  on top of the codebase-agnostic ones?
- Default chosen if no answer: Started small with 2 per bucket so the
  popover stays scannable. Every scenario is fully codebase-agnostic
  (refers to "the longest function", "the README", "the hottest path"
  rather than language-specific symbols). Operators can edit any field
  after applying a scenario, so they don't have to wait for a perfectly
  matching preset.
- Files affected: `web/src/tasks/test-scenarios/testScenarios.ts`
  (`TEST_SCENARIOS`).
- Resolution (2026-04-29): Confirmed start-small default. Scaling the
  catalog has zero technical cost — `TEST_SCENARIOS` is a typed array,
  any future agent can append entries. Shipping 10 scannable scenarios
  is better than shipping 40 nobody reads through; the bar to add more
  is only "an operator asks for one we don't have".

### Test scenarios do not overwrite project / runner / model / schedule
- Date: 2026-04-29
- From task: Pre-defined test scenarios
- Decision needed: Picking a scenario fills Title / Prompt / Priority /
  Task type / Checklist but leaves Project / Runner / Model / Schedule /
  Pending subtasks alone. Is that the intended boundary, or should
  scenarios also force a particular runner / model so test runs are
  reproducible?
- Default chosen if no answer: Leave the runtime configuration alone.
  An operator who has already configured "always run against project X
  with model Y" should not have those wiped by picking a scenario. If
  reproducible runtime presets become a requirement, fold them in by
  adding optional `runner` / `cursorModel` fields to the `TestScenario`
  type and apply them only when set.
- Files affected: `web/src/tasks/hooks/useTaskCreateFlow.ts`
  (`applyTestScenario`).
- Resolution (2026-04-29): Confirmed runtime-untouched default. The
  principle: a "fill the form for me" affordance must never silently
  redirect the run to a different runner / project. If reproducible
  presets become a requirement, the type already has room to grow
  optional `runner` / `cursorModel` fields applied only when set.

### Stats strip lives inside the task list panel, not at the page top
- Date: 2026-04-29
- From task: All Tasks page polish
- Decision needed: The new `TaskListStatsStrip` (total / ready /
  critical / scheduled / review / blocked) renders between the heading
  and the filters inside the same panel that hosts the table. Should
  it instead sit as its own page-level "scoreboard" panel above the
  list?
- Default chosen if no answer: Inside the panel. Keeps the page to a
  single column with one anchored container; the strip is a tight
  one-line summary, not a dashboard, and doesn't need its own
  elevation. Self-hides on `total === 0` so a fresh database still
  shows the welcome empty state without a confusing "0 ready" pill.
- Files affected:
  `web/src/tasks/components/task-list/section/TaskListStatsStrip.tsx`,
  `web/src/tasks/components/task-list/section/TaskListSection.tsx`.
- Resolution (2026-04-29): Confirmed inside-panel placement. Lifting the
  strip to its own page-level scoreboard would imply this app has a
  dashboard (it doesn't — All Tasks IS the dashboard). The strip is row
  metadata for the table below, so it belongs inside the same panel that
  owns the table.

### Empty-state title kept verbatim "No tasks yet"
- Date: 2026-04-29
- From task: All Tasks page polish
- Decision needed: The empty state's title was almost reworded to
  "Ready when you are." for warmth. Reverted to "No tasks yet" because
  many `App.test.tsx` integration tests use that literal as their
  page-loaded sentinel. Should we still rename it (and update the
  ~12 test sites that depend on it) for friendlier copy?
- Default chosen if no answer: Keep the literal title; refresh only
  the description (now reads "Hit New task to dispatch your first run.
  Once a task is in flight, this table tracks its status, priority,
  and prompt preview live as the worker picks it up."). Lower risk,
  same warmth gain.
- Files affected:
  `web/src/tasks/components/task-list/table/TaskListDataTable.tsx`,
  `web/src/app/styles/task-list/app-task-list-controls.css`
  (`.empty-state--task-list-fresh`).
- Resolution (2026-04-29): Confirmed literal title. "No tasks yet" is
  factual, simple, and clear — Linear / Notion / GitHub use the same
  cadence. The warmth gain comes from the inviting description, not
  from cuter title copy. Avoiding ~12 test churn for a copy nuance is
  the right trade.

### Settings page lede copy is `tune --runtime --workspace --agent`
- Date: 2026-04-29
- From task: Settings page polish
- Decision needed: The new one-line terminal lede underneath the
  page subtitle reads `$ tune --runtime --workspace --agent`. Should
  it instead be `$ configure --runtime` / `$ admin --runtime` / a
  single-flag `$ settings --tune`? The pattern matches the create
  modal (`$ compose --next-up`) and the All Tasks page
  (`$ query --next-up --filter --review`) — three "verb plus the
  flags this surface acts on" phrases.
- Default chosen if no answer: `tune` because it is the verb most
  consistent with what this page does (turn knobs on the live system),
  and the flags map 1:1 to the actual fieldset section names so the
  lede doubles as a table-of-contents.
- Files affected: `web/src/settings/SettingsSections.tsx`
  (`.settings-page-lede`).
- Resolution (2026-04-29): Confirmed `tune --runtime --workspace --agent`.
  `configure` is a noun-leaning verb; `admin` reads bureaucratic; a
  single flag would lose the table-of-contents value. `tune` keeps the
  page feeling like a live instrument the operator adjusts.

### Settings fieldsets keep separate cards rather than collapsing into one panel
- Date: 2026-04-29
- From task: Settings page polish
- Decision needed: The All Tasks page and the New Task modal are each
  rendered as a single `.panel` chrome wrapping all their content. The
  Settings page deliberately keeps its multiple fieldset cards (Agent
  worker / Display / Workspace / Cursor agent / Run timeout). Should
  they be merged into one panel for total parity?
- Default chosen if no answer: Keep the separate cards. Apple System
  Settings, Stripe Dashboard settings, and Linear preferences all
  group settings into discrete cards because each section is
  independently navigable, scroll-anchorable (`/settings#cursor-agent`
  works), and saveable in isolation. Merging them would lose the
  per-section brand-tinted left rail / focus-within affordance, hurt
  scroll-into-view, and crowd the page. Visual continuity with the
  other surfaces comes from the shared brand top-strip + radial wash
  on the page itself, not from a single panel chrome.
- Files affected: `web/src/settings/settings.css` (`.settings-page`,
  `.settings-fieldset`).
- Resolution (2026-04-29): Confirmed separate cards. The
  `/settings#cursor-agent` deep-link contract alone justifies discrete
  fieldsets — collapsing them into one panel would either (a) break the
  scroll-into-view affordance or (b) require synthetic anchors that
  don't match the visual structure. Card-per-section is the right model.

### Settings page section glyphs use `$` (not `>`) to match the form intent
- Date: 2026-04-29
- From task: Settings page polish
- Decision needed: The page heading uses `term-arrow` (`>`) and the
  fieldset legends use `term-prompt` (`$`). Should the legends use
  the same arrow as the page heading for tighter heading hierarchy?
- Default chosen if no answer: `$` for legends because the create
  modal headers and All Tasks section heads also use `$` — the `>` is
  reserved for top-of-page H2 only. Mixing `>` in legend positions
  would visually flatten the H2 → fieldset hierarchy and make the
  page read like a bullet list instead of a section group.
- Files affected: `web/src/settings/SettingsSections.tsx` (legends),
  `web/src/settings/settings.css` (`.settings-fieldset > .settings-fieldset-legend.term-prompt`).
- Resolution (2026-04-29): Confirmed `$` for legends, `>` reserved for
  H2. The two-glyph hierarchy (`>` for the page, `$` for sections) is
  the same vocabulary every other polished surface uses; flattening it
  would degrade the heading rhythm across the whole app, not just on
  Settings.

### Drafts page reuses `task-list-section-panel` chrome rather than its own
- Date: 2026-04-29
- From task: Drafts page polish
- Decision needed: The drafts page wraps its content in `panel
  task-list-section-panel` so it inherits the brand-tinted top
  accent strip, the soft vertical gradient, the lifted `$` glyph,
  and the staggered fade-in defined for the All Tasks page. Should
  it instead get its own `task-drafts-section-panel` modifier and a
  fully-independent CSS surface?
- Default chosen if no answer: Reuse. The rules in
  `app-task-list-controls.css` are framed as "section panel polish
  — mirrors the create-modal treatment so the two surfaces feel
  like siblings". Drafts is a third sibling under the same parent
  shell pattern; copying the CSS into a `task-drafts-section-panel`
  twin would just be drift waiting to happen. New drafts-only
  rules live in the dedicated partial
  `web/src/app/styles/task-drafts/app-task-drafts.css` so the
  shared shell stays generic.
- Files affected: `web/src/tasks/pages/TaskDraftsPage.tsx`,
  `web/src/app/styles/task-drafts/app-task-drafts.css`,
  `web/src/app/App.css`.
- Resolution (2026-04-29): Confirmed reuse. Forking
  `task-list-section-panel` into a near-identical `task-drafts-section-panel`
  twin is the kind of CSS drift that compounds for years; a shared shell
  with feature-specific partials is the canonical pattern. If the Drafts
  surface ever needs chrome the All Tasks page does NOT need, the
  modifier can be added then — until then, less surface = less drift.

### Drafts row exposes title + relative timestamp, not just buttons
- Date: 2026-04-29
- From task: Drafts page polish
- Decision needed: The legacy row was two flat buttons —
  `Resume: <name>` / `Delete` — with the title baked into the
  Resume button's label. The polished row lifts the title into a
  dedicated `.draft-row__name` cell with an `Edited <relative>`
  sub-label. Should the title also stay in the Resume button text,
  or is the dedicated cell enough?
- Default chosen if no answer: Dedicated cell only; the Resume
  button now reads simply `Resume`. The button still carries the
  full `aria-label="Open draft <name> in create form"` (preserved
  from the previous implementation so all `App.test.tsx` queries
  by accessible-name keep passing). Visible duplication added
  noise without adding scannability — the title cell is more
  prominent and the timestamp is the new useful piece.
- Files affected: `web/src/tasks/pages/TaskDraftsPage.tsx`
  (Resume / Delete button cluster + meta cell layout).
- Resolution (2026-04-29): Confirmed dedicated cell. The title belongs
  to the row, not to the button — every list view in Stripe / Linear /
  Notion separates the row's identity from its actions. The `aria-label`
  preserves accessible-name continuity so screen readers still hear
  "Open draft <name> in create form" when the button takes focus.

### `formatRelativeTime` lives under `web/src/shared/time/`
- Date: 2026-04-29
- From task: Drafts page polish
- Decision needed: The drafts page needs an "Edited 5 min ago"
  affordance and there is no existing relative-time helper. Should
  the helper live as a one-off utility inside the drafts page, or
  as a shared module under `web/src/shared/time/`?
- Default chosen if no answer: Shared module. Stripe / Linear /
  Apple settings UI all use the same compact relative-time
  formatting in dozens of surfaces (audit rows, recent activity,
  last-updated chips). Keeping it shared from day one means the
  next caller (Settings "Last saved" chip, task list updated_at
  column, audit timeline) doesn't need to invent another variant.
  The helper has its own test suite (`relativeTime.test.ts`,
  10 tests) and ships with explicit bucket boundaries, future-time
  collapse, and unparseable-input tolerance.
- Files affected: `web/src/shared/time/relativeTime.ts`,
  `web/src/shared/time/relativeTime.test.ts`,
  `web/src/tasks/pages/TaskDraftsPage.tsx`.
- Resolution (2026-04-29): Confirmed shared module. Inlining as a
  drafts-only utility would guarantee the next caller invents a
  divergent twin (different bucket boundaries, different "just now"
  threshold, different unparseable-input handling). The shared module
  with full test coverage is the cheap-now / pays-forever path.

### Drafts row does not surface project / runner / task-type metadata
- Date: 2026-04-29
- From task: Drafts page polish
- Decision needed: A draft can carry `project_id`,
  `project_context_item_ids`, runner / model selection, schedule,
  priority, task type, etc. — none of which are visible on the
  drafts list (only name and updated_at). Should the row also
  surface project / priority / task-type pills?
- Default chosen if no answer: Stay with name + relative time
  only. The drafts page is a "pick one to resume" surface, not a
  comparison surface. Adding pills would (a) require a richer
  `TaskDraftSummary` shape from the backend (currently
  `id/name/created_at/updated_at`), (b) crowd the row, and (c)
  duplicate state the operator will see the moment they Resume
  into the create modal. If a user reports "I can't tell my drafts
  apart", revisit by adding a `payload_summary` to the draft list
  endpoint and surfacing one or two pills here.
- Files affected: `web/src/tasks/pages/TaskDraftsPage.tsx`.
- Resolution (2026-04-29): Confirmed minimal row. Adding pills now
  would either ship them empty (the backend doesn't return the data
  yet) or force a backend change for a UI that nobody has asked to be
  richer. The doc trail above is enough that a future operator can
  retrace the path if the need ever surfaces.

### Navbar uses an underline accent under the active item AND the existing brand pill
- Date: 2026-04-29
- From task: Navbar polish
- Decision needed: The active nav item now carries both the existing
  brand-tinted pill (background + border + brand-color text) AND a
  small `::after` underline indicator below it (Stripe.com top-nav
  vocabulary). Should we collapse to one cue (pill OR underline) for
  a quieter look?
- Default chosen if no answer: Keep both. The pill alone made the
  active state read as "this is a button" rather than "you are
  here"; the underline alone (no pill) would lose the `aria-current`
  affordance for users who can't perceive thin marks. Two
  complementary cues — color tint + underline — pairs the Apple
  pill with the Stripe rule, and stays accessible without color.
- Files affected: `web/src/app/styles/base/app-shell.css`
  (`.app-nav__link[aria-current="page"]::after`).
- Resolution (2026-04-29): Confirmed two-cue active state. Pill alone
  reads as a button, underline alone fails operators who can't perceive
  thin marks; the pair (Apple pill + Stripe underline) is the
  established WCAG-friendly pattern for "you are here".

### Sticky header lifts to `--shadow-md` once scrolled past 4px
- Date: 2026-04-29
- From task: Navbar polish
- Decision needed: A new `useStickyShellElevation` hook toggles
  `data-elevated="true"` on the sticky header once `window.scrollY`
  passes 4px. The header then lifts to `--shadow-md` plus a hairline
  brand-tinted top edge. Is 4px the right threshold, or should it be
  more conservative (e.g. 16–24px) so the elevation only kicks in
  after a deliberate scroll?
- Default chosen if no answer: 4px. Matches Stripe / Linear / Notion
  dashboard top-nav behavior — the elevation appears the moment the
  page edge passes under the sticky frame, so the chrome reads as
  "floating" without ambiguity. Higher thresholds delay the cue
  enough that operators feel "is this stuck or floating?" briefly,
  which is the affordance the elevation is meant to resolve.
- Files affected: `web/src/lib/useStickyShellElevation.ts`,
  `web/src/lib/useStickyShellElevation.test.ts`,
  `web/src/app/App.tsx`,
  `web/src/app/styles/base/app-shell.css`
  (`.app-header--sticky[data-elevated="true"]`).
- Resolution (2026-04-29): Confirmed 4px threshold. Industry standard
  (Stripe / Linear / Notion); higher thresholds create the
  "is it stuck or floating?" ambiguity the elevation is supposed to
  resolve. The threshold is a hook parameter so any future caller can
  override per-surface.

### Settings cog rotates 35° on hover
- Date: 2026-04-29
- From task: Navbar polish
- Decision needed: The settings cog gains a `transform: rotate(35deg)`
  on hover (Apple-style affordance signaling "this control is
  interactive"). Should it rotate further (e.g. 90° / 180°), spin
  continuously, or not move at all?
- Default chosen if no answer: 35° subtle quarter-rotation under the
  shared `--ease-out` easing, disabled by `prefers-reduced-motion`.
  Anything more becomes playful (Slack-style bounce) and out of
  character with the rest of the calm, terminal-inflected aesthetic.
- Files affected: `web/src/settings/settings.css`
  (`.app-header-settings-link:hover .app-header-settings-icon`).
- Resolution (2026-04-29): Confirmed 35° subtle rotation. The whole
  app's vocabulary is "calm, terminal-inflected" — bouncy or spinning
  controls would feel stylistically out of place. 35° is enough to
  read as "this moves on hover" without crossing into Slack /
  marketing-site territory.
