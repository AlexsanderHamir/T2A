/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_TASKAPI_ORIGIN?: string;
  readonly VITE_TASK_GRAPH_MOCK_URL?: string;
  /** When "true" or "1", the SPA serves synthetic tasks/projects payloads for layout review. */
  readonly VITE_UI_TEST_MODE?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
