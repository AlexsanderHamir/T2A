import type { GitBranch, GitLiveBranch, GitLiveWorktree, GitRepository, GitWorktree, GitWorktreeBranchBind, WorktreeBranch } from "@/types/git";
import type { ProjectListResponse } from "@/types/project";
import { parseProjectListResponse } from "./projects";
import {
  parseGitBranchList,
  parseGitLiveBranchList,
  parseGitLiveWorktreeList,
  parseGitRepository,
  parseGitRepositoryList,
  parseGitWorktree,
  parseGitWorktreeList,
  parseWorktreeBranch,
  parseWorktreeBranchList,
} from "./parseGitApi";
import { assertTaskPathId } from "./taskRequestBounds";
import { apiErrorFromResponse, fetchWithTimeout, jsonHeaders } from "./shared";

const gitRoot = "/git";

export async function listGlobalGitRepositories(
  options?: { signal?: AbortSignal },
): Promise<GitRepository[]> {
  const res = await fetchWithTimeout(`${gitRoot}/repositories`, {
    headers: { Accept: "application/json" },
    signal: options?.signal,
  });
  if (!res.ok) throw await apiErrorFromResponse(res);
  return parseGitRepositoryList((await res.json()) as unknown);
}

export async function createGlobalGitRepository(input: {
  path: string;
  host_path?: string;
  default_branch?: string;
}): Promise<GitRepository> {
  const res = await fetchWithTimeout(`${gitRoot}/repositories`, {
    method: "POST",
    headers: jsonHeaders,
    body: JSON.stringify(input),
  });
  if (!res.ok) throw await apiErrorFromResponse(res);
  return parseGitRepository((await res.json()) as unknown);
}

export async function getGlobalGitRepository(
  repositoryId: string,
  options?: { signal?: AbortSignal },
): Promise<GitRepository> {
  const repoId = assertTaskPathId(repositoryId, "repository id");
  const res = await fetchWithTimeout(`${gitRoot}/repositories/${encodeURIComponent(repoId)}`, {
    headers: { Accept: "application/json" },
    signal: options?.signal,
  });
  if (!res.ok) throw await apiErrorFromResponse(res);
  return parseGitRepository((await res.json()) as unknown);
}

export async function deleteGlobalGitRepository(repositoryId: string): Promise<void> {
  const repoId = assertTaskPathId(repositoryId, "repository id");
  const res = await fetchWithTimeout(`${gitRoot}/repositories/${encodeURIComponent(repoId)}`, {
    method: "DELETE",
  });
  if (!res.ok) throw await apiErrorFromResponse(res);
}

export async function listGlobalGitWorktrees(
  repositoryId: string,
  options?: { signal?: AbortSignal },
): Promise<GitWorktree[]> {
  const repoId = assertTaskPathId(repositoryId, "repository id");
  const res = await fetchWithTimeout(
    `${gitRoot}/repositories/${encodeURIComponent(repoId)}/worktrees`,
    { headers: { Accept: "application/json" }, signal: options?.signal },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
  return parseGitWorktreeList((await res.json()) as unknown);
}

export async function createGlobalGitWorktree(
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
    `${gitRoot}/repositories/${encodeURIComponent(repoId)}/worktrees`,
    { method: "POST", headers: jsonHeaders, body: JSON.stringify(input) },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
  return parseGitWorktree((await res.json()) as unknown);
}

export async function registerGlobalGitWorktree(
  repositoryId: string,
  input: {
    path: string;
    name?: string;
    branch?: GitWorktreeBranchBind;
  },
): Promise<GitWorktree> {
  const repoId = assertTaskPathId(repositoryId, "repository id");
  const res = await fetchWithTimeout(
    `${gitRoot}/repositories/${encodeURIComponent(repoId)}/worktrees/register`,
    { method: "POST", headers: jsonHeaders, body: JSON.stringify(input) },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
  return parseGitWorktree((await res.json()) as unknown);
}

export async function deleteGlobalGitWorktree(
  worktreeId: string,
  options?: { force?: boolean },
): Promise<void> {
  const wtId = assertTaskPathId(worktreeId, "worktree id");
  const params = new URLSearchParams();
  if (options?.force) params.set("force", "true");
  const qs = params.toString();
  const res = await fetchWithTimeout(
    `${gitRoot}/worktrees/${encodeURIComponent(wtId)}${qs ? `?${qs}` : ""}`,
    { method: "DELETE" },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
}

export async function listGlobalGitBranches(
  repositoryId: string,
  options?: { signal?: AbortSignal },
): Promise<GitBranch[]> {
  const repoId = assertTaskPathId(repositoryId, "repository id");
  const res = await fetchWithTimeout(
    `${gitRoot}/repositories/${encodeURIComponent(repoId)}/branches`,
    { headers: { Accept: "application/json" }, signal: options?.signal },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
  return parseGitBranchList((await res.json()) as unknown);
}

export async function listGlobalGitLiveWorktrees(
  repositoryId: string,
  options?: { signal?: AbortSignal },
): Promise<GitLiveWorktree[]> {
  const repoId = assertTaskPathId(repositoryId, "repository id");
  const res = await fetchWithTimeout(
    `${gitRoot}/repositories/${encodeURIComponent(repoId)}/worktrees/live`,
    { headers: { Accept: "application/json" }, signal: options?.signal },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
  return parseGitLiveWorktreeList((await res.json()) as unknown);
}

export async function listGlobalGitLiveBranches(
  repositoryId: string,
  options?: { signal?: AbortSignal },
): Promise<GitLiveBranch[]> {
  const repoId = assertTaskPathId(repositoryId, "repository id");
  const res = await fetchWithTimeout(
    `${gitRoot}/repositories/${encodeURIComponent(repoId)}/branches/live`,
    { headers: { Accept: "application/json" }, signal: options?.signal },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
  return parseGitLiveBranchList((await res.json()) as unknown);
}

export async function listWorktreeBranchAssociations(
  worktreeId: string,
  options?: { signal?: AbortSignal },
): Promise<WorktreeBranch[]> {
  const wtId = assertTaskPathId(worktreeId, "worktree id");
  const res = await fetchWithTimeout(
    `${gitRoot}/worktrees/${encodeURIComponent(wtId)}/branches`,
    { headers: { Accept: "application/json" }, signal: options?.signal },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
  return parseWorktreeBranchList((await res.json()) as unknown);
}

export async function associateWorktreeBranch(
  worktreeId: string,
  input: { branch_id?: string; name?: string; start_point?: string; create_branch?: boolean },
): Promise<WorktreeBranch> {
  const wtId = assertTaskPathId(worktreeId, "worktree id");
  const res = await fetchWithTimeout(
    `${gitRoot}/worktrees/${encodeURIComponent(wtId)}/branches`,
    { method: "POST", headers: jsonHeaders, body: JSON.stringify(input) },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
  return parseWorktreeBranch((await res.json()) as unknown);
}

export async function removeWorktreeBranchAssociation(
  worktreeId: string,
  branchId: string,
): Promise<void> {
  const wtId = assertTaskPathId(worktreeId, "worktree id");
  const brId = assertTaskPathId(branchId, "branch id");
  const res = await fetchWithTimeout(
    `${gitRoot}/worktrees/${encodeURIComponent(wtId)}/branches/${encodeURIComponent(brId)}`,
    { method: "DELETE" },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
}

export async function reconcileGlobalGitRepository(
  repositoryId: string,
): Promise<import("@/types/git").GitReconcileResult> {
  const repoId = assertTaskPathId(repositoryId, "repository id");
  const res = await fetchWithTimeout(
    `${gitRoot}/repositories/${encodeURIComponent(repoId)}/reconcile`,
    { method: "POST", headers: jsonHeaders, body: "{}" },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
  const { parseGitReconcileResult } = await import("./parseGitApi");
  return parseGitReconcileResult((await res.json()) as unknown);
}

export async function listProjectsByRepository(
  repositoryId: string,
  options?: { signal?: AbortSignal },
): Promise<ProjectListResponse> {
  const repoId = assertTaskPathId(repositoryId, "repository id");
  const res = await fetchWithTimeout(
    `${gitRoot}/repositories/${encodeURIComponent(repoId)}/projects`,
    { headers: { Accept: "application/json" }, signal: options?.signal },
  );
  if (!res.ok) throw await apiErrorFromResponse(res);
  return parseProjectListResponse((await res.json()) as unknown);
}
