/// <reference types="vite/client" />

interface ImportMetaEnv {
  readonly VITE_TASKAPI_ORIGIN?: string;
  readonly VITE_TASK_GRAPH_MOCK_URL?: string;
}

interface ImportMeta {
  readonly env: ImportMetaEnv;
}
