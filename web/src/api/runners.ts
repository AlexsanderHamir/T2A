import { fetchWithTimeout, jsonHeaders, apiErrorFromResponse } from "./shared";

// ---------------------------------------------------------------------------
// Types
// ---------------------------------------------------------------------------

export type RunnerConfigField = {
  key: string;
  label: string;
  type: "string" | "secret" | "int" | "bool" | "enum";
  default?: unknown;
  help?: string;
  required?: boolean;
  sensitive?: boolean;
  enum_values?: Array<{ value: string; label: string }>;
};

export type RunnerConfigSchema = {
  version: number;
  fields: RunnerConfigField[];
};

export type RunnerDescriptor = {
  id: string;
  label: string;
  default_binary_hint: string;
  config_schema?: RunnerConfigSchema;
};

export type RunnerProbeResult = {
  ok: boolean;
  runner: string;
  binary_path?: string;
  version?: string;
  error?: string;
};

export type RunnerListModelsResult = {
  ok: boolean;
  runner: string;
  binary_path?: string;
  models?: Array<{ id: string; label: string }>;
  error?: string;
};

export type RunnerValidateConfigResult = {
  valid: boolean;
  error?: string;
};

// ---------------------------------------------------------------------------
// Fetchers
// ---------------------------------------------------------------------------

export async function fetchRunners(
  options?: { signal?: AbortSignal },
): Promise<RunnerDescriptor[]> {
  const res = await fetchWithTimeout("/runners", {
    headers: { Accept: "application/json" },
    signal: options?.signal,
  });
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw: unknown = await res.json();
  if (!Array.isArray(raw)) {
    throw new Error("unexpected /runners response shape");
  }
  return raw.map(assertRunnerDescriptor);
}

export async function fetchRunnerConfigSchema(
  runnerId: string,
  options?: { signal?: AbortSignal },
): Promise<RunnerConfigSchema> {
  const res = await fetchWithTimeout(`/runners/${encodeURIComponent(runnerId)}/config-schema`, {
    headers: { Accept: "application/json" },
    signal: options?.signal,
  });
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw: unknown = await res.json();
  return assertConfigSchema(raw);
}

export async function probeRunner(
  runnerId: string,
  body?: { binary_path?: string },
  options?: { signal?: AbortSignal },
): Promise<RunnerProbeResult> {
  const res = await fetchWithTimeout(`/runners/${encodeURIComponent(runnerId)}/probe`, {
    method: "POST",
    headers: jsonHeaders,
    body: body ? JSON.stringify(body) : undefined,
    signal: options?.signal,
  });
  if (!res.ok && res.status !== 404 && res.status !== 501) {
    throw await apiErrorFromResponse(res);
  }
  const raw: unknown = await res.json();
  return assertProbeResult(raw);
}

export async function listRunnerModels(
  runnerId: string,
  body?: { binary_path?: string },
  options?: { signal?: AbortSignal },
): Promise<RunnerListModelsResult> {
  const res = await fetchWithTimeout(`/runners/${encodeURIComponent(runnerId)}/list-models`, {
    method: "POST",
    headers: jsonHeaders,
    body: body ? JSON.stringify(body) : undefined,
    signal: options?.signal,
  });
  if (!res.ok && res.status !== 404 && res.status !== 501) {
    throw await apiErrorFromResponse(res);
  }
  const raw: unknown = await res.json();
  return assertListModelsResult(raw);
}

export async function validateRunnerConfig(
  runnerId: string,
  config: Record<string, unknown>,
  options?: { signal?: AbortSignal },
): Promise<RunnerValidateConfigResult> {
  const res = await fetchWithTimeout(
    `/runners/${encodeURIComponent(runnerId)}/validate-config`,
    {
      method: "POST",
      headers: jsonHeaders,
      body: JSON.stringify(config),
      signal: options?.signal,
    },
  );
  if (!res.ok && res.status !== 422) {
    throw await apiErrorFromResponse(res);
  }
  const raw: unknown = await res.json();
  if (raw === null || typeof raw !== "object") {
    throw new Error("unexpected validate-config response");
  }
  const o = raw as Record<string, unknown>;
  const out: RunnerValidateConfigResult = {
    valid: typeof o.valid === "boolean" ? o.valid : false,
  };
  if (typeof o.error === "string") out.error = o.error;
  return out;
}

// ---------------------------------------------------------------------------
// Assertion helpers
// ---------------------------------------------------------------------------

function assertRunnerDescriptor(raw: unknown): RunnerDescriptor {
  if (raw === null || typeof raw !== "object") {
    throw new Error("unexpected runner descriptor shape");
  }
  const o = raw as Record<string, unknown>;
  if (typeof o.id !== "string" || typeof o.label !== "string") {
    throw new Error("unexpected runner descriptor shape");
  }
  const out: RunnerDescriptor = {
    id: o.id,
    label: o.label,
    default_binary_hint: typeof o.default_binary_hint === "string" ? o.default_binary_hint : "",
  };
  if (o.config_schema !== undefined && o.config_schema !== null) {
    out.config_schema = assertConfigSchema(o.config_schema);
  }
  return out;
}

function assertConfigSchema(raw: unknown): RunnerConfigSchema {
  if (raw === null || typeof raw !== "object") {
    throw new Error("unexpected config schema shape");
  }
  const o = raw as Record<string, unknown>;
  if (typeof o.version !== "number" || !Array.isArray(o.fields)) {
    throw new Error("unexpected config schema shape");
  }
  return {
    version: o.version,
    fields: o.fields.map(assertConfigField),
  };
}

function assertConfigField(raw: unknown): RunnerConfigField {
  if (raw === null || typeof raw !== "object") {
    throw new Error("unexpected config field shape");
  }
  const o = raw as Record<string, unknown>;
  if (typeof o.key !== "string" || typeof o.label !== "string" || typeof o.type !== "string") {
    throw new Error("unexpected config field shape");
  }
  const field: RunnerConfigField = {
    key: o.key,
    label: o.label,
    type: o.type as RunnerConfigField["type"],
  };
  if (o.default !== undefined) field.default = o.default;
  if (typeof o.help === "string") field.help = o.help;
  if (typeof o.required === "boolean") field.required = o.required;
  if (typeof o.sensitive === "boolean") field.sensitive = o.sensitive;
  if (Array.isArray(o.enum_values)) {
    field.enum_values = o.enum_values
      .filter((v): v is Record<string, unknown> => v !== null && typeof v === "object")
      .map((v) => ({
        value: typeof v.value === "string" ? v.value : "",
        label: typeof v.label === "string" ? v.label : "",
      }));
  }
  return field;
}

function assertProbeResult(raw: unknown): RunnerProbeResult {
  if (raw === null || typeof raw !== "object") {
    throw new Error("unexpected probe response");
  }
  const o = raw as Record<string, unknown>;
  const out: RunnerProbeResult = {
    ok: typeof o.ok === "boolean" ? o.ok : false,
    runner: typeof o.runner === "string" ? o.runner : "",
  };
  if (typeof o.binary_path === "string") out.binary_path = o.binary_path;
  if (typeof o.version === "string") out.version = o.version;
  if (typeof o.error === "string") out.error = o.error;
  return out;
}

function assertListModelsResult(raw: unknown): RunnerListModelsResult {
  if (raw === null || typeof raw !== "object") {
    throw new Error("unexpected list-models response");
  }
  const o = raw as Record<string, unknown>;
  const out: RunnerListModelsResult = {
    ok: typeof o.ok === "boolean" ? o.ok : false,
    runner: typeof o.runner === "string" ? o.runner : "",
  };
  if (typeof o.binary_path === "string") out.binary_path = o.binary_path;
  if (typeof o.error === "string") out.error = o.error;
  if (Array.isArray(o.models)) {
    out.models = o.models
      .filter((m): m is Record<string, unknown> => m !== null && typeof m === "object")
      .map((m) => ({
        id: typeof m.id === "string" ? m.id : "",
        label: typeof m.label === "string" ? m.label : "",
      }));
  }
  return out;
}
