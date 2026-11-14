import { describe, expect, it } from "vitest";
import { render } from "@testing-library/react";

import { a11yViolations } from "../axe";

// The pass is only worth having if it fails on the shapes the board's notes
// described, so each is shown here with its named form passing.
describe("the accessibility pass", () => {
  it("fails a dialog with no accessible name (w4/607)", async () => {
    const { container } = render(
      <div role="dialog">
        <p>Filter by status</p>
      </div>,
    );
    const found = await a11yViolations(container);
    expect(found.join("\n")).toMatch(/aria-dialog-name/);
  });

  it("passes the same dialog once it is named", async () => {
    const { container } = render(
      <div role="dialog" aria-label="Filter by status">
        <p>Filter by status</p>
      </div>,
    );
    expect(await a11yViolations(container)).toEqual([]);
  });

  it("fails a link whose aria-label replaces its visible text (w4/612)", async () => {
    const { container } = render(
      <a href="/docs" aria-label="Open the platform guide">
        Docs
      </a>,
    );
    const found = await a11yViolations(container);
    expect(found.join("\n")).toMatch(/label-in-name/);
  });

  it("passes a label that contains the visible text, and an icon-only control", async () => {
    const { container } = render(
      <div>
        <a href="/docs" aria-label="Docs: open the platform guide">
          Docs
        </a>
        <button aria-label="Close">×</button>
      </div>,
    );
    expect(await a11yViolations(container)).toEqual([]);
  });

  it("fails an unnamed button, an image with no alt and an unlabelled field", async () => {
    const { container } = render(
      <form>
        <button type="button" />
        <img src="/x.png" />
        <input type="text" />
      </form>,
    );
    const found = (await a11yViolations(container)).join("\n");
    expect(found).toMatch(/button-name/);
    expect(found).toMatch(/image-alt/);
    expect(found).toMatch(/\blabel\b/);
  });

  it("compares Chinese text the same way", async () => {
    const { container } = render(<button aria-label="关闭对话框">取消</button>);
    expect((await a11yViolations(container)).join("\n")).toMatch(
      /label-in-name/,
    );
  });
});
