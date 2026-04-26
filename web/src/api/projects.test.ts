import { afterEach, describe, expect, it, vi } from "vitest";
import {
  createProjectContext,
  deleteProjectContext,
  getProject,
  listProjectContext,
  listProjects,
  patchProject,
  patchProjectContext,
  parseProject,
  parseProjectContextListResponse,
} from "./projects";

const projectWire = {
  id: "11111111-1111-4111-8111-111111111111",
  name: "Context moat",
  description: "Long-running work",
  status: "active",
  context_summary: "Shared memory",
  created_at: "2026-04-26T00:00:00Z",
  updated_at: "2026-04-26T00:00:00Z",
};

describe("project API parsers", () => {
  afterEach(() => {
    vi.restoreAllMocks();
  });

  it("parses project rows", () => {
    expect(parseProject(projectWire).name).toBe("Context moat");
  });

  it("rejects unknown project statuses", () => {
    expect(() => parseProject({ ...projectWire, status: "paused" })).toThrow(
      /project status/,
    );
  });

  it("lists projects with query params", async () => {
    const spy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ projects: [projectWire], limit: 10 }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const out = await listProjects({ limit: 10, includeArchived: true });

    expect(out.projects).toHaveLength(1);
    expect(String(spy.mock.calls[0][0])).toContain("include_archived=true");
  });

  it("gets one project", async () => {
    vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify(projectWire), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    await expect(getProject(projectWire.id)).resolves.toMatchObject({
      id: projectWire.id,
    });
  });

  it("patches projects", async () => {
    const spy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ ...projectWire, name: "Renamed" }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    const out = await patchProject(projectWire.id, { name: "Renamed" });

    expect(out.name).toBe("Renamed");
    expect(spy.mock.calls[0][1]).toMatchObject({ method: "PATCH" });
  });

  it("parses project context list rows", () => {
    const out = parseProjectContextListResponse({
      items: [
        {
          id: "ctx-1",
          project_id: projectWire.id,
          kind: "decision",
          title: "Use relational context",
          body: "Defer embeddings.",
          created_by: "user",
          pinned: true,
          created_at: "2026-04-26T00:00:00Z",
          updated_at: "2026-04-26T00:00:00Z",
        },
      ],
      limit: 50,
    });

    expect(out.items[0].kind).toBe("decision");
  });

  it("lists project context", async () => {
    const spy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ items: [], limit: 5 }), {
        status: 200,
        headers: { "Content-Type": "application/json" },
      }),
    );

    await listProjectContext(projectWire.id, { limit: 5, pinnedOnly: true });

    expect(String(spy.mock.calls[0][0])).toContain("pinned_only=true");
  });

  it("creates, patches, and deletes project context", async () => {
    const itemWire = {
      id: "22222222-2222-4222-8222-222222222222",
      project_id: projectWire.id,
      kind: "note",
      title: "Remember",
      body: "Keep context explicit.",
      created_by: "user",
      pinned: false,
      created_at: "2026-04-26T00:00:00Z",
      updated_at: "2026-04-26T00:00:00Z",
    };
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        new Response(JSON.stringify(itemWire), {
          status: 201,
          headers: { "Content-Type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ ...itemWire, pinned: true }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(new Response(null, { status: 204 }));

    await createProjectContext(projectWire.id, {
      kind: "note",
      title: "Remember",
      body: "Keep context explicit.",
      pinned: false,
    });
    await patchProjectContext(projectWire.id, itemWire.id, { pinned: true });
    await deleteProjectContext(projectWire.id, itemWire.id);

    expect(spy.mock.calls.map((call) => call[1]?.method)).toEqual([
      "POST",
      "PATCH",
      "DELETE",
    ]);
  });
});
