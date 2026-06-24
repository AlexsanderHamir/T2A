import { fetchWithTimeout, apiErrorFromResponse } from "./shared";

export type WorkspaceBrowseCategory =
  | "registered"
  | "install"
  | "home"
  | "documents"
  | "desktop"
  | "downloads"
  | "pictures"
  | "music"
  | "videos"
  | "custom";

export type WorkspaceBrowseRoot = {
  id: string;
  path: string;
  label: string;
  category?: WorkspaceBrowseCategory;
  available: boolean;
  unavailable_reason?: string;
};

export type WorkspaceRootsResponse = {
  roots: WorkspaceBrowseRoot[];
  environment: "native" | "docker";
};

export type BrowseDirEntry = {
  name: string;
  path: string;
  has_children: boolean;
  is_git_repo: boolean;
};

export type BrowseDirsResponse = {
  path?: string;
  parent_path?: string;
  entries: BrowseDirEntry[];
};

function parseBrowseCategory(raw: unknown): WorkspaceBrowseCategory | undefined {
  if (typeof raw !== "string" || raw === "") {
    return undefined;
  }
  const allowed: WorkspaceBrowseCategory[] = [
    "registered",
    "install",
    "home",
    "documents",
    "desktop",
    "downloads",
    "pictures",
    "music",
    "videos",
    "custom",
  ];
  return allowed.includes(raw as WorkspaceBrowseCategory)
    ? (raw as WorkspaceBrowseCategory)
    : undefined;
}

function parseBrowseRoot(raw: unknown): WorkspaceBrowseRoot {
  if (typeof raw !== "object" || raw === null) {
    throw new Error("invalid browse root");
  }
  const value = raw as Record<string, unknown>;
  return {
    id: typeof value.id === "string" ? value.id : "",
    path: typeof value.path === "string" ? value.path : "",
    label: typeof value.label === "string" ? value.label : "",
    category: parseBrowseCategory(value.category),
    available: value.available === true,
    unavailable_reason:
      typeof value.unavailable_reason === "string"
        ? value.unavailable_reason
        : undefined,
  };
}

function parseBrowseDirEntry(raw: unknown): BrowseDirEntry {
  if (typeof raw !== "object" || raw === null) {
    throw new Error("invalid browse dir entry");
  }
  const value = raw as Record<string, unknown>;
  return {
    name: typeof value.name === "string" ? value.name : "",
    path: typeof value.path === "string" ? value.path : "",
    has_children: value.has_children === true,
    is_git_repo: value.is_git_repo === true,
  };
}

export function parseWorkspaceRootsResponse(raw: unknown): WorkspaceRootsResponse {
  if (typeof raw !== "object" || raw === null) {
    throw new Error("invalid workspace roots response");
  }
  const value = raw as Record<string, unknown>;
  const rootsRaw = value.roots;
  if (!Array.isArray(rootsRaw)) {
    throw new Error("workspace roots missing roots array");
  }
  const environment = value.environment;
  if (environment !== "native" && environment !== "docker") {
    throw new Error("workspace roots missing environment");
  }
  return {
    roots: rootsRaw.map(parseBrowseRoot),
    environment,
  };
}

export function parseBrowseDirsResponse(raw: unknown): BrowseDirsResponse {
  if (typeof raw !== "object" || raw === null) {
    throw new Error("invalid browse dirs response");
  }
  const value = raw as Record<string, unknown>;
  const entriesRaw = value.entries;
  if (!Array.isArray(entriesRaw)) {
    throw new Error("browse dirs missing entries array");
  }
  return {
    path: typeof value.path === "string" ? value.path : undefined,
    parent_path:
      typeof value.parent_path === "string" ? value.parent_path : undefined,
    entries: entriesRaw.map(parseBrowseDirEntry),
  };
}

export async function fetchWorkspaceRoots(
  init?: RequestInit,
): Promise<WorkspaceRootsResponse> {
  const res = await fetchWithTimeout("/settings/workspace-roots", {
    ...init,
    headers: { Accept: "application/json", ...(init?.headers ?? {}) },
  });
  if (!res.ok) {
    throw await apiErrorFromResponse(res);
  }
  return parseWorkspaceRootsResponse(await res.json());
}

export async function browseWorkspaceDirs(
  path?: string,
  init?: RequestInit,
): Promise<BrowseDirsResponse> {
  const params = new URLSearchParams();
  if (path && path.trim() !== "") {
    params.set("path", path);
  }
  const qs = params.toString();
  const url = qs ? `/settings/browse-dirs?${qs}` : "/settings/browse-dirs";
  const res = await fetchWithTimeout(url, {
    ...init,
    headers: { Accept: "application/json", ...(init?.headers ?? {}) },
  });
  if (!res.ok) {
    throw await apiErrorFromResponse(res);
  }
  return parseBrowseDirsResponse(await res.json());
}
