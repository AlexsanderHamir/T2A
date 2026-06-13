export const TASK_TIMINGS = {
  listRefreshShowMs: 380,
  listRefreshHideMs: 520,
  draftAutosaveDebounceMs: 900,
  draftResumeMinLoadingMs: 300,
  draftDeleteExitMs: 180,
} as const;

export const TASK_DRAFTS: {
  untitledDraftName: string;
  createModalDraftListLimit: number;
  draftsPageDefaultLimit: number;
  resumeModalPerPage: number;
} = {
  untitledDraftName: "Untitled draft",
  createModalDraftListLimit: 100,
  draftsPageDefaultLimit: 50,
  resumeModalPerPage: 5,
};
