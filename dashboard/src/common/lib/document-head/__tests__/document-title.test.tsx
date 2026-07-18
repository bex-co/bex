import { describe, it, expect, beforeEach } from "vitest";
import { render, waitFor } from "@testing-library/react";
import { DashboardDocumentTitle } from "../document-title";
import { DASHBOARD_NAME } from "../constants";

const ERROR_TITLE = `Something went wrong ・ ${DASHBOARD_NAME}`;

function addHeadTitle(text: string) {
  const el = document.createElement("title");
  el.textContent = text;
  document.head.appendChild(el);
  return el;
}

beforeEach(() => {
  document.head.querySelectorAll("title").forEach((el) => el.remove());
});

describe("DashboardDocumentTitle unmount reconciliation (w1/m52, inbox 036)", () => {
  it("resets the tab when only the fallback title survives unmount", async () => {
    // The SSR-orphan scenario: a hydration bail leaves the server-hoisted
    // error <title> behind as an unmanaged head tag.
    addHeadTitle(ERROR_TITLE);
    const view = render(<DashboardDocumentTitle title={ERROR_TITLE} />);
    view.unmount();
    await waitFor(() => expect(document.title).toBe(DASHBOARD_NAME));
    expect(document.head.querySelectorAll("title")).toHaveLength(1);
  });

  it("keeps a recovering route's own title and drops the stale copies", async () => {
    addHeadTitle(ERROR_TITLE); // SSR orphan
    const view = render(<DashboardDocumentTitle title={ERROR_TITLE} />);
    // The recovered route emitted its real title.
    addHeadTitle(`my-project ・ ${DASHBOARD_NAME}`);
    view.unmount();
    await waitFor(() =>
      expect(document.title).toBe(`my-project ・ ${DASHBOARD_NAME}`),
    );
    expect(
      Array.from(document.head.querySelectorAll("title")).map(
        (el) => el.textContent,
      ),
    ).toEqual([`my-project ・ ${DASHBOARD_NAME}`]);
  });

  it("an immediate remount supersedes the outgoing cleanup (StrictMode)", async () => {
    const view = render(<DashboardDocumentTitle title={ERROR_TITLE} />);
    view.unmount();
    // Remount before the deferred reconcile fires — the fallback page is
    // still the active page and must keep its title.
    render(<DashboardDocumentTitle title={ERROR_TITLE} />);
    await new Promise((r) => setTimeout(r, 20));
    const titles = Array.from(document.head.querySelectorAll("title")).map(
      (el) => el.textContent,
    );
    expect(titles).toContain(ERROR_TITLE);
  });
});
