import { describe, expect, it } from "vitest";
import { isUiFeatureOmitted, OMITTED_UI_FEATURES } from "./omittedFeatures";

describe("omittedFeatures", () => {
  it("documents projects as omitted for the current launch", () => {
    expect(OMITTED_UI_FEATURES.projects).toBe(true);
    expect(isUiFeatureOmitted("projects")).toBe(true);
  });

  it("documents tags and dependencies as omitted for the current launch", () => {
    expect(OMITTED_UI_FEATURES.tagsAndDependencies).toBe(true);
    expect(isUiFeatureOmitted("tagsAndDependencies")).toBe(true);
  });

  it("documents schedule as omitted for the current launch", () => {
    expect(OMITTED_UI_FEATURES.schedule).toBe(true);
    expect(isUiFeatureOmitted("schedule")).toBe(true);
  });
});
