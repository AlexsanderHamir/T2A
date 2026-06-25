import {
  queryOptions,
  useQuery,
  type QueryClient,
} from "@tanstack/react-query";
import {
  browseWorkspaceDirs,
  fetchWorkspaceRoots,
  type WorkspaceBrowseRoot,
  type WorkspaceRootsResponse,
} from "@/api/settingsBrowse";
import { QUERY_POLICY } from "@/tasks/queryPolicy";
import { settingsQueryKeys } from "../queryKeys";

export type WorkspaceRootsView = {
  environment: WorkspaceRootsResponse["environment"];
  roots: WorkspaceBrowseRoot[];
};

function mapWorkspaceRootsPayload(payload: WorkspaceRootsResponse): WorkspaceRootsView {
  return { environment: payload.environment, roots: payload.roots };
}

export function workspaceRootsQueryOptions() {
  return queryOptions({
    queryKey: settingsQueryKeys.workspaceRoots(),
    queryFn: async ({ signal }) =>
      mapWorkspaceRootsPayload(await fetchWorkspaceRoots({ signal })),
    staleTime: QUERY_POLICY.shellStaleTimeMs,
  });
}

export function browseDirsQueryOptions(path: string) {
  const trimmed = path.trim();
  return queryOptions({
    queryKey: settingsQueryKeys.browseDirs(trimmed),
    queryFn: async ({ signal }) => browseWorkspaceDirs(trimmed, { signal }),
    staleTime: QUERY_POLICY.shellStaleTimeMs,
  });
}

export function useWorkspaceRoots(options?: { enabled?: boolean }) {
  return useQuery({
    ...workspaceRootsQueryOptions(),
    enabled: options?.enabled !== false,
  });
}

export function prefetchWorkspaceRoots(queryClient: QueryClient): void {
  void queryClient.prefetchQuery(workspaceRootsQueryOptions());
}

export function prefetchBrowseDirs(queryClient: QueryClient, path: string): void {
  if (path.trim() === "") return;
  void queryClient.prefetchQuery(browseDirsQueryOptions(path));
}

/** Roots plus the first available starting location listing (typical first click). */
export function prefetchWorkspacePickerShell(queryClient: QueryClient): void {
  void (async () => {
    try {
      const roots = await queryClient.fetchQuery(workspaceRootsQueryOptions());
      const firstAvailable = roots.roots.find((root) => root.available);
      if (firstAvailable) {
        prefetchBrowseDirs(queryClient, firstAvailable.path);
      }
    } catch {
      // Prefetch is a perf hint — 404, abort on unmount, and network errors
      // must not surface as unhandled rejections.
    }
  })();
}
