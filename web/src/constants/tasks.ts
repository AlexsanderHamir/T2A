export const TASK_TIMINGS = {
  listRefreshShowMs: 380,
  listRefreshHideMs: 520,
  draftAutosaveDebounceMs: 900,
  draftResumeMinLoadingMs: 300,
} as const;

export const TASK_DRAFTS: {
  initialDmapCommitLimit: string;
  untitledDraftName: string;
  createModalDraftListLimit: number;
  draftsPageDefaultLimit: number;
  resumeModalPerPage: number;
} = {
  initialDmapCommitLimit: "5",
  untitledDraftName: "Untitled draft",
  createModalDraftListLimit: 100,
  draftsPageDefaultLimit: 50,
  resumeModalPerPage: 5,
};
