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
  is_git_repo?: boolean;
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
          roots: [{ id: "home", path: "/roots", label: "Home", category: "home", available: true }],
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
            is_git_repo: true,
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
          roots: [{ id: "home", path: "/roots", label: "Home", category: "home", available: true }],
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

  it("groups roots into workspace and user folder sections", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = String(input);
      if (url.endsWith("/settings/workspace-roots")) {
        return jsonResponse({
          environment: "native",
          roots: [
            { id: "install", path: "/app", label: "Hamix checkout", category: "install", available: true },
            { id: "home", path: "/roots", label: "Home", category: "home", available: true },
            {
              id: "documents",
              path: "/roots/Documents",
              label: "Documents",
              category: "documents",
              available: true,
            },
          ],
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

    expect(await screen.findByText("Workspace")).toBeInTheDocument();
    expect(screen.getByText("User folders")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Hamix checkout/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^Documents/ })).toBeInTheDocument();
    fetchMock.mockRestore();
  });

  it("renders bootstrap entry points when workspace-roots returns OS places", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = String(input);
      if (url.endsWith("/settings/workspace-roots")) {
        return jsonResponse({
          environment: "native",
          roots: [
            {
              id: "install",
              path: "/app",
              label: "Hamix checkout",
              category: "install",
              available: true,
            },
            { id: "home", path: "/roots", label: "Home", category: "home", available: true },
          ],
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

    expect(await screen.findByRole("button", { name: /Home/ })).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /Hamix checkout/ })).toBeInTheDocument();
    fetchMock.mockRestore();
  });

  it("disables the confirm button when no folder is open yet", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = String(input);
      if (url.endsWith("/settings/workspace-roots")) {
        return jsonResponse({
          environment: "native",
          roots: [{ id: "home", path: "/roots", label: "Home", category: "home", available: true }],
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

  it("returns to starting locations when Back is pressed at a browse root", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = String(input);
      if (url.endsWith("/settings/workspace-roots")) {
        return jsonResponse({
          environment: "native",
          roots: [
            {
              id: "home",
              path: "/roots",
              label: "Home",
              category: "home",
              available: true,
            },
            {
              id: "documents",
              path: "/roots/OneDrive/Documents",
              label: "Documents",
              category: "documents",
              available: true,
            },
          ],
        });
      }
      if (url.includes("/settings/browse-dirs")) {
        return browseRouter({
          "/roots/OneDrive/Documents": {
            path: "/roots/OneDrive/Documents",
            parent_path: "/roots/OneDrive",
            entries: [
              {
                name: "Hamix",
                path: "/roots/OneDrive/Documents/Hamix",
                has_children: false,
                is_git_repo: true,
              },
            ],
          },
          "/roots/OneDrive/Documents/Hamix": {
            path: "/roots/OneDrive/Documents/Hamix",
            parent_path: "/roots/OneDrive/Documents",
            is_git_repo: true,
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
        onSelect={() => {}}
      />,
    );

    await userEvent.click(await screen.findByRole("button", { name: /^Documents/ }));
    expect(
      screen.getByRole("button", { name: /Back to starting locations/ }),
    ).toBeInTheDocument();
    expect(screen.queryByText("Choose a folder to browse from")).not.toBeInTheDocument();

    await userEvent.click(screen.getByRole("button", { name: /Back to starting locations/ }));

    expect(await screen.findByText("Choose a folder to browse from")).toBeInTheDocument();
    expect(screen.getByRole("button", { name: /^Documents/ })).toBeInTheDocument();
    expect(screen.queryByRole("button", { name: /Back/ })).not.toBeInTheDocument();
    fetchMock.mockRestore();
  });

  it("steps up one folder when Back is pressed inside a browse root", async () => {
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = String(input);
      if (url.endsWith("/settings/workspace-roots")) {
        return jsonResponse({
          environment: "native",
          roots: [
            {
              id: "documents",
              path: "/roots/OneDrive/Documents",
              label: "Documents",
              category: "documents",
              available: true,
            },
          ],
        });
      }
      if (url.includes("/settings/browse-dirs")) {
        return browseRouter({
          "/roots/OneDrive/Documents": {
            path: "/roots/OneDrive/Documents",
            parent_path: "/roots/OneDrive",
            entries: [
              {
                name: "Hamix",
                path: "/roots/OneDrive/Documents/Hamix",
                has_children: false,
                is_git_repo: true,
              },
            ],
          },
          "/roots/OneDrive/Documents/Hamix": {
            path: "/roots/OneDrive/Documents/Hamix",
            parent_path: "/roots/OneDrive/Documents",
            is_git_repo: true,
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
        onSelect={() => {}}
      />,
    );

    await userEvent.click(await screen.findByRole("button", { name: /^Documents/ }));
    await userEvent.click(await screen.findByRole("button", { name: /Hamix/ }));

    await waitFor(() => {
      expect(screen.getByText("/roots/OneDrive/Documents/Hamix")).toBeInTheDocument();
    });

    await userEvent.click(screen.getByRole("button", { name: /Go up one folder/ }));

    await waitFor(() => {
      expect(screen.getByText("/roots/OneDrive/Documents")).toBeInTheDocument();
    });
    expect(screen.getByRole("button", { name: /Hamix/ })).toBeInTheDocument();
    expect(screen.queryByText("Choose a folder to browse from")).not.toBeInTheDocument();
    fetchMock.mockRestore();
  });

  it("blocks confirmation when requireGitRepository and folder is not a git checkout", async () => {
    const onSelect = vi.fn();
    const fetchMock = vi.spyOn(globalThis, "fetch").mockImplementation(async (input) => {
      const url = String(input);
      if (url.endsWith("/settings/workspace-roots")) {
        return jsonResponse({
          environment: "native",
          roots: [{ id: "home", path: "/roots", label: "Home", category: "home", available: true }],
        });
      }
      if (url.includes("/settings/browse-dirs")) {
        return browseRouter({
          "/roots": {
            path: "/roots",
            parent_path: "",
            is_git_repo: false,
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
            is_git_repo: true,
            entries: [],
          },
        })(url);
      }
      return new Response("not found", { status: 404 });
    });

    render(
      <WorkspaceDirPickerModal
        open
        requireGitRepository
        currentPath=""
        onClose={() => {}}
        onSelect={onSelect}
      />,
    );

    await userEvent.click(await screen.findByRole("button", { name: /Home/ }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /Use this folder/ })).toBeDisabled();
    });

    await userEvent.click(screen.getByRole("button", { name: /my-app/ }));

    await waitFor(() => {
      expect(screen.getByRole("button", { name: /Use this folder/ })).toBeEnabled();
    });

    await userEvent.click(screen.getByRole("button", { name: /Use this folder/ }));
    expect(onSelect).toHaveBeenCalledWith("/roots/my-app");
    fetchMock.mockRestore();
  });
});
