import { render } from "@testing-library/react";
import { describe, expect, it } from "vitest";
import { RichPromptMenuBar } from "./RichPromptMenuBar";

describe("RichPromptMenuBar", () => {
  it("renders nothing when editor is null", () => {
    const { container } = render(<RichPromptMenuBar editor={null} />);
    expect(container.firstChild).toBeNull();
  });
});
