import { render, screen, waitFor } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { describe, expect, it, vi } from "vitest";
import { WorkspaceDirPickerModal } from "./WorkspaceDirPickerModal";

function jsonResponse(body: unknown): Response {
  return new Response(JSON.stringify(body), {
    status: 200,
    headers: { "content-type": "application/json" },
  });
}

type BrowseFixture = {
  path: string;
  parent_path: string;
  entries: Array<{
    name: string;
    path: string;
    has_children: boolean;
    is_git_repo: boolean;
  }>;
};

function browseRouter(fixtures: Record<string, BrowseFixture>) {
  return (rawUrl: string): Response => {
    const u = new URL(rawUrl, "http://test.local");
    const p = u.searchParams.get("path") ?? "";
    const fx = fixtures[p];
    if (!fx) {
      return new Response("not found", { status: 404 });
    }
    return jsonResponse(fx);
  };
}

describe("WorkspaceDirPickerModal", () => {
  it("opens a starting location, navigates into a folder, and confirms its path", async () => {
    const onSelect = vi.fn();
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = String(input);
      if (url.endsWith("/settings/workspace-roots")) {
        return jsonResponse({
          environment: "native",
          roots: [{ id: "home", path: "/roots", label: "Home", available: true }],
        });
      }
      if (url.includes("/settings/browse-dirs")) {
        return browseRouter({
          "/roots": {
            path: "/roots",
            parent_path: "",
            entries: [
              {
                name: "my-app",
                path: "/roots/my-app",
                has_children: false,
                is_git_repo: true,
              },
            ],
          },
          "/roots/my-app": {
            path: "/roots/my-app",
            parent_path: "/roots",
            entries: [],
          },
        })(url);
      }
      return new Response("not found", { status: 404 });
    });

    render(
      <WorkspaceDirPickerModal
        open
        currentPath=""
        onClose={() => {}}
        onSelect={onSelect}
      />,
    );

    // Step into the Home root.
    await userEvent.click(await screen.findByRole("button", { name: /Home/ }));

    // Step into the folder inside Home.
    await userEvent.click(await screen.findByRole("button", { name: /my-app/ }));

    // Footer reflects the new location and the primary action is enabled.
    await waitFor(() => {
      expect(screen.getByText("/roots/my-app")).toBeInTheDocument();
    });

    await userEvent.click(screen.getByRole("button", { name: /Use this folder/ }));
    expect(onSelect).toHaveBeenCalledWith("/roots/my-app");
    fetchMock.mockRestore();
  });

  it("can register the currently-open folder even when it has no subfolders", async () => {
    const onSelect = vi.fn();
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = String(input);
      if (url.endsWith("/settings/workspace-roots")) {
        return jsonResponse({
          environment: "native",
          roots: [{ id: "home", path: "/roots", label: "Home", available: true }],
        });
      }
      if (url.includes("/settings/browse-dirs")) {
        return browseRouter({
          "/roots": { path: "/roots", parent_path: "", entries: [] },
        })(url);
      }
      return new Response("not found", { status: 404 });
    });

    render(
      <WorkspaceDirPickerModal
        open
        currentPath=""
        onClose={() => {}}
        onSelect={onSelect}
      />,
    );

    await userEvent.click(await screen.findByRole("button", { name: /Home/ }));

    await waitFor(() => {
      expect(
        screen.getByText(/No subfolders inside this folder/),
      ).toBeInTheDocument();
    });

    await userEvent.click(screen.getByRole("button", { name: /Use this folder/ }));
    expect(onSelect).toHaveBeenCalledWith("/roots");
    fetchMock.mockRestore();
  });

  it("disables the confirm button when no folder is open yet", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = String(input);
      if (url.endsWith("/settings/workspace-roots")) {
        return jsonResponse({
          environment: "native",
          roots: [{ id: "home", path: "/roots", label: "Home", available: true }],
        });
      }
      return new Response("not found", { status: 404 });
    });

    render(
      <WorkspaceDirPickerModal
        open
        currentPath=""
        onClose={() => {}}
        onSelect={() => {}}
      />,
    );

    expect(await screen.findByRole("button", { name: /Use this folder/ })).toBeDisabled();
    fetchMock.mockRestore();
  });
});
