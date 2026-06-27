import {
  globalGitBranchesResponse,
  globalGitLiveBranchesResponse,
  globalGitRepositoriesResponse,
  globalGitWorktreesResponse,
} from "../factories/git";

/** Responds to global `/git/*` REST paths (ADR-0037). */
export function respondGlobalGitApi(url: string, method = "GET"): Response | null {
  const base = "/git";
  if (method === "GET") {
    if (url.endsWith(`${base}/repositories`)) {
      return Response.json(globalGitRepositoriesResponse());
    }
    if (url.includes(`${base}/repositories/`) && url.endsWith("/worktrees/probe")) {
      const probePath = new URL(url, "http://local").searchParams.get("path") ?? "";
      const linked = probePath.includes("/repo/");
      return Response.json({
        path: probePath || "/repo/wt-feature",
        linked,
        is_main: false,
        branch: linked ? "feature" : "",
        registered: false,
      });
    }
    if (url.includes(`${base}/repositories/`) && url.endsWith("/worktrees/live")) {
      return Response.json({
        worktrees: [
          {
            path: "/repo/main",
            branch: "main",
            is_main: true,
            detached: false,
            registered: false,
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
