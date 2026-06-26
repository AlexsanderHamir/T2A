import {
  globalGitBranchesResponse,
  globalGitLiveBranchesResponse,
  globalGitRepositoriesResponse,
  globalGitWorktreesResponse,
  worktreeBranchAssociationsResponse,
} from "../factories/git";

/** Responds to global `/git/*` REST paths (ADR-0037). */
export function respondGlobalGitApi(url: string, method = "GET"): Response | null {
  const base = "/git";
  if (method === "GET") {
    if (url.endsWith(`${base}/repositories`)) {
      return Response.json(globalGitRepositoriesResponse());
    }
    if (url.includes(`${base}/repositories/`) && url.endsWith("/worktrees/live")) {
      return Response.json({
        worktrees: [
          {
            path: "/repo/main",
            branch: "main",
            is_main: true,
            detached: false,
            registered: true,
          },
        ],
      });
    }
    if (url.includes(`${base}/repositories/`) && url.endsWith("/worktrees")) {
      return Response.json(globalGitWorktreesResponse());
    }
    if (url.includes(`${base}/repositories/`) && url.endsWith("/branches/live")) {
      return Response.json(globalGitLiveBranchesResponse());
    }
    if (url.includes(`${base}/repositories/`) && url.endsWith("/branches")) {
      return Response.json(globalGitBranchesResponse());
    }
    if (url.includes(`${base}/worktrees/`) && url.endsWith("/branches")) {
      return Response.json(worktreeBranchAssociationsResponse());
    }
    if (url.includes(`${base}/repositories/`) && url.endsWith("/projects")) {
      return Response.json({ projects: [], limit: 100 });
    }
  }
  if (method === "POST" && url.endsWith(`${base}/repositories`)) {
    const body = globalGitRepositoriesResponse() as { repositories: unknown[] };
    return Response.json(body.repositories[0], { status: 201 });
  }
  return null;
}
