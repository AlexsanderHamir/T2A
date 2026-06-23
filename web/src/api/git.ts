import type { GitBranch, GitReconcileResult, GitRepository, GitWorktree } from "@/types/git";
import {
  parseGitBranch,
  parseGitBranchList,
  parseGitReconcileResult,
  parseGitRepository,
  parseGitRepositoryList,
  parseGitWorktree,
  parseGitWorktreeList,
} from "./parseGitApi";
import { assertTaskPathId } from "./taskRequestBounds";
import { apiErrorFromResponse, fetchWithTimeout, jsonHeaders } from "./shared";

function gitBase(projectId: string): string {
  const pid = assertTaskPathId(projectId, "project id");
  return `/projects/${encodeURIComponent(pid)}/git`;
}

export async function listGitRepositories(
  projectId: string,
  options?: { signal?: AbortSignal },
): Promise<GitRepository[]> {
  const res = await fetchWithTimeout(`${gitBase(projectId)}/repositories`, {
    headers: { Accept: "application/json" },
    signal: options?.signal,
  });
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw: unknown = await res.json();
  return parseGitRepositoryList(raw);
}

export async function createGitRepository(
  projectId: string,
  input: { path: string; host_path?: string; default_branch?: string },
): Promise<GitRepository> {
  const res = await fetchWithTimeout(`${gitBase(projectId)}/repositories`, {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify(input),
  });
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw: unknown = await res.json();
  return parseGitRepository(raw);
}

export async function deleteGitRepository(
  projectId: string,
  repositoryId: string,
): Promise<void> {
  const repoId = assertTaskPathId(repositoryId, "repository id");
  const res = await fetchWithTimeout(
    `${gitBase(projectId)}/repositories/${encodeURIComponent(repoId)}`,
    { method: "DELETE" },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
}

export async function listGitWorktrees(
  projectId: string,
  repositoryId: string,
  options?: { signal?: AbortSignal },
): Promise<GitWorktree[]> {
  const repoId = assertTaskPathId(repositoryId, "repository id");
  const res = await fetchWithTimeout(
    `${gitBase(projectId)}/repositories/${encodeURIComponent(repoId)}/worktrees`,
    {
      headers: { Accept: "application/json" },
      signal: options?.signal,
    },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw: unknown = await res.json();
  return parseGitWorktreeList(raw);
}

export async function createGitWorktree(
  projectId: string,
  repositoryId: string,
  input: {
    path: string;
    name?: string;
    branch: string;
    create_branch?: boolean;
    start_point?: string;
  },
): Promise<GitWorktree> {
  const repoId = assertTaskPathId(repositoryId, "repository id");
  const res = await fetchWithTimeout(
    `${gitBase(projectId)}/repositories/${encodeURIComponent(repoId)}/worktrees`,
    {
      method: "POST",
      headers: jsonHeaders,
      body: JSON.stringify(input),
    },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw: unknown = await res.json();
  return parseGitWorktree(raw);
}

export async function deleteGitWorktree(
  projectId: string,
  worktreeId: string,
  options?: { force?: boolean },
): Promise<void> {
  const wtId = assertTaskPathId(worktreeId, "worktree id");
  const params = new URLSearchParams();
  if (options?.force) params.set("force", "true");
  const qs = params.toString();
  const res = await fetchWithTimeout(
    `${gitBase(projectId)}/worktrees/${encodeURIComponent(wtId)}${qs ? `?${qs}` : ""}`,
    { method: "DELETE" },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
}

export async function listGitBranches(
  projectId: string,
  repositoryId: string,
  options?: { signal?: AbortSignal },
): Promise<GitBranch[]> {
  const repoId = assertTaskPathId(repositoryId, "repository id");
  const res = await fetchWithTimeout(
    `${gitBase(projectId)}/repositories/${encodeURIComponent(repoId)}/branches`,
    {
      headers: { Accept: "application/json" },
      signal: options?.signal,
    },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw: unknown = await res.json();
  return parseGitBranchList(raw);
}

export async function createGitBranch(
  projectId: string,
  repositoryId: string,
  input: { name: string; start_point?: string },
): Promise<GitBranch> {
  const repoId = assertTaskPathId(repositoryId, "repository id");
  const res = await fetchWithTimeout(
    `${gitBase(projectId)}/repositories/${encodeURIComponent(repoId)}/branches`,
    {
      method: "POST",
      headers: jsonHeaders,
      body: JSON.stringify(input),
    },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw: unknown = await res.json();
  return parseGitBranch(raw);
}

export async function deleteGitBranch(
  projectId: string,
  branchId: string,
  options?: { force?: boolean },
): Promise<void> {
  const bid = assertTaskPathId(branchId, "branch id");
  const params = new URLSearchParams();
  if (options?.force) params.set("force", "true");
  const qs = params.toString();
  const res = await fetchWithTimeout(
    `${gitBase(projectId)}/branches/${encodeURIComponent(bid)}${qs ? `?${qs}` : ""}`,
    { method: "DELETE" },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
}

export async function reconcileGitRepository(
  projectId: string,
  repositoryId: string,
): Promise<GitReconcileResult> {
  const repoId = assertTaskPathId(repositoryId, "repository id");
  const res = await fetchWithTimeout(
    `${gitBase(projectId)}/repositories/${encodeURIComponent(repoId)}/reconcile`,
    { method: "POST", headers: jsonHeaders, body: "{}" },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
  const raw: unknown = await res.json();
  return parseGitReconcileResult(raw);
}
