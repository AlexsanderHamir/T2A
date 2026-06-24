import { http, HttpResponse } from "msw";
import { DEFAULT_PROJECT_ID } from "@/types";
import {
  gitBranchesResponse,
  gitRepositoriesResponse,
  gitWorktreesResponse,
} from "./git";
import {
  globalGitBranchesResponse,
  globalGitLiveBranchesResponse,
  globalGitRepositoriesResponse,
  globalGitWorktreesResponse,
  worktreeBranchAssociationsResponse,
} from "../factories/git";

/** MSW handlers for project-scoped git REST paths. */
export function gitApiHandlers() {
  const base = `/projects/${DEFAULT_PROJECT_ID}/git`;
  return [
    http.get(`${base}/repositories`, () =>
      HttpResponse.json(gitRepositoriesResponse()),
    ),
    http.get(new RegExp(`${base}/repositories/.+/worktrees`), () =>
      HttpResponse.json(gitWorktreesResponse()),
    ),
    http.get(new RegExp(`${base}/repositories/.+/branches`), () =>
      HttpResponse.json(gitBranchesResponse()),
    ),
  ];
}

/** MSW handlers for global `/git/*` REST paths. */
export function globalGitApiHandlers() {
  return [
    http.get("/git/repositories", () =>
      HttpResponse.json(globalGitRepositoriesResponse()),
    ),
    http.get(new RegExp("/git/repositories/.+/worktrees"), () =>
      HttpResponse.json(globalGitWorktreesResponse()),
    ),
    http.get(new RegExp("/git/repositories/.+/branches/live"), () =>
      HttpResponse.json(globalGitLiveBranchesResponse()),
    ),
    http.get(new RegExp("/git/repositories/.+/branches"), () =>
      HttpResponse.json(globalGitBranchesResponse()),
    ),
    http.get(new RegExp("/git/worktrees/.+/branches"), () =>
      HttpResponse.json(worktreeBranchAssociationsResponse()),
    ),
    http.get(new RegExp("/git/repositories/.+/projects"), () =>
      HttpResponse.json({ projects: [], limit: 100 }),
    ),
  ];
}
