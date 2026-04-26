import { afterEach, describe, expect, it, vi } from "vitest";
import {
  createProjectContext,
  createProjectContextEdge,
  deleteProjectContext,
  deleteProjectContextEdge,
  getProject,
  listProjectContext,
  listProjects,
  patchProject,
  patchProjectContext,
  patchProjectContextEdge,
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
      edges: [
        {
          id: "33333333-3333-4333-8333-333333333333",
          project_id: projectWire.id,
          source_context_id: "ctx-1",
          target_context_id: "ctx-2",
          relation: "supports",
          strength: 4,
          note: "Decision supports constraint",
          created_at: "2026-04-26T00:00:00Z",
          updated_at: "2026-04-26T00:00:00Z",
        },
      ],
      limit: 50,
    });

    expect(out.items[0].kind).toBe("decision");
    expect(out.edges[0].relation).toBe("supports");
  });

  it("defaults missing project context edges to an empty list", () => {
    const out = parseProjectContextListResponse({
      items: [],
      limit: 50,
    });

    expect(out.edges).toEqual([]);
  });

  it("lists project context", async () => {
    const spy = vi.spyOn(globalThis, "fetch").mockResolvedValue(
      new Response(JSON.stringify({ items: [], edges: [], limit: 5 }), {
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

  it("creates, patches, and deletes project context edges", async () => {
    const edgeWire = {
      id: "33333333-3333-4333-8333-333333333333",
      project_id: projectWire.id,
      source_context_id: "11111111-1111-4111-8111-111111111111",
      target_context_id: "22222222-2222-4222-8222-222222222222",
      relation: "related",
      strength: 3,
      note: "",
      created_at: "2026-04-26T00:00:00Z",
      updated_at: "2026-04-26T00:00:00Z",
    };
    const spy = vi
      .spyOn(globalThis, "fetch")
      .mockResolvedValueOnce(
        new Response(JSON.stringify(edgeWire), {
          status: 201,
          headers: { "Content-Type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(
        new Response(JSON.stringify({ ...edgeWire, strength: 5 }), {
          status: 200,
          headers: { "Content-Type": "application/json" },
        }),
      )
      .mockResolvedValueOnce(new Response(null, { status: 204 }));

    await createProjectContextEdge(projectWire.id, {
      source_context_id: edgeWire.source_context_id,
      target_context_id: edgeWire.target_context_id,
      relation: "related",
      strength: 3,
    });
    await patchProjectContextEdge(projectWire.id, edgeWire.id, { strength: 5 });
    await deleteProjectContextEdge(projectWire.id, edgeWire.id);

    expect(spy.mock.calls.map((call) => call[1]?.method)).toEqual([
      "POST",
      "PATCH",
      "DELETE",
    ]);
  });
});
