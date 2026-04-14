import { describe, expect, it } from "vitest";
import { TASK_EVENTS_PAGE_SIZE, TASK_LIST_PAGE_SIZE } from "./paging";

describe("paging", () => {
  it("keeps list and events page sizes aligned with server defaults (docs/API-HTTP.md)", () => {
    expect(TASK_LIST_PAGE_SIZE).toBe(20);
    expect(TASK_EVENTS_PAGE_SIZE).toBe(20);
    expect(TASK_LIST_PAGE_SIZE).toBe(TASK_EVENTS_PAGE_SIZE);
  });
});
