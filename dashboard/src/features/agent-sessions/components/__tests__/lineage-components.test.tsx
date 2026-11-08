import { describe, it, expect } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import {
  RouterProvider,
  createRootRoute,
  createRoute,
  createRouter,
  createMemoryHistory,
} from "@tanstack/react-router";
import type { ReactNode } from "react";
import { AgentsPanel } from "@/features/agent-sessions/components/agents-panel";
import { ChildSessionCards } from "@/features/agent-sessions/components/child-session-cards";
import { agentSessionView } from "@/test/mocks/agent-session";

function renderWithRouter(ui: ReactNode) {
  const root = createRootRoute();
  const route = createRoute({
    getParentRoute: () => root,
    path: "/",
    component: () => <>{ui}</>,
  });
  const router = createRouter({
    routeTree: root.addChildren([route]),
    history: createMemoryHistory({ initialEntries: ["/"] }),
    context: { client: {} as never, session: null },
  });
  return render(<RouterProvider router={router} />);
}

const done = agentSessionView({
  id: "c1",
  title: "Write the migration",
  phase: "completed",
  state: "succeeded",
});
const working = agentSessionView({
  id: "c2",
  title: "Add the endpoint",
  phase: "running",
  state: "working",
});

describe("ChildSessionCards", () => {
  it("renders one linked card per child, in the given order, with a status", async () => {
    renderWithRouter(<ChildSessionCards childSessions={[done, working]} />);
    const block = await screen.findByTestId("child-session-cards");
    const links = within(block).getAllByRole("link");
    expect(links.map((l) => l.getAttribute("href"))).toEqual([
      "/agents/c1",
      "/agents/c2",
    ]);
    expect(links[0]).toHaveTextContent("Write the migration");
    expect(links[0]).toHaveTextContent("Done");
    expect(links[1]).toHaveTextContent("Working");
    expect(within(block).getByText("Child sessions")).toBeInTheDocument();
  });

  it("renders nothing without children", () => {
    const { container } = render(<ChildSessionCards childSessions={[]} />);
    expect(container).toBeEmptyDOMElement();
  });
});

describe("AgentsPanel", () => {
  it("groups descendants by status with a count and View session links", async () => {
    renderWithRouter(
      <AgentsPanel
        descendants={[working, done]}
        parentSessionId={null}
        loading={false}
      />,
    );
    const panel = await screen.findByTestId("agents-panel");
    const groups = panel.querySelectorAll("[data-status-group]");
    expect([...groups].map((g) => g.getAttribute("data-status-group"))).toEqual(
      ["working", "done"],
    );
    expect(within(panel).getByText("Done 1")).toBeInTheDocument();
    const view = within(panel).getAllByRole("link", { name: "View session" });
    expect(view.map((l) => l.getAttribute("href"))).toEqual([
      "/agents/c2",
      "/agents/c1",
    ]);
  });

  it("shows every status group with its count, in Working, Done, Failed, Canceled order", async () => {
    const failed = agentSessionView({ id: "c3", phase: "failed", state: "failed" });
    const canceled = agentSessionView({ id: "c4", phase: "canceled", state: "canceled" });
    const done2 = agentSessionView({ id: "c5", phase: "completed", state: "succeeded" });
    renderWithRouter(
      <AgentsPanel
        descendants={[canceled, done, failed, working, done2]}
        parentSessionId={null}
        loading={false}
      />,
    );
    const panel = await screen.findByTestId("agents-panel");
    const headings = [...panel.querySelectorAll("h3")].map((h) => h.textContent);
    expect(headings).toEqual(["Working 1", "Done 2", "Failed 1", "Canceled 1"]);
  });

  it("links a child's panel back to its parent and collapses", async () => {
    renderWithRouter(
      <AgentsPanel descendants={[]} parentSessionId="p1" loading={false} />,
    );
    const panel = await screen.findByTestId("agents-panel");
    expect(
      within(panel).getByRole("link", { name: "Parent session" }),
    ).toHaveAttribute("href", "/agents/p1");
    await userEvent.click(
      within(panel).getByRole("button", { name: "Collapse the Agents panel" }),
    );
    expect(screen.queryByTestId("agents-panel-body")).not.toBeInTheDocument();
    expect(
      within(panel).getByRole("button", { name: "Expand the Agents panel" }),
    ).toHaveAttribute("aria-expanded", "false");
  });

  it("holds the layout while the tree loads: skeleton row shares the real row height", async () => {
    const { unmount } = renderWithRouter(
      <AgentsPanel descendants={[]} parentSessionId="p1" loading />,
    );
    const skeleton = await screen.findByTestId("agents-panel-skeleton");
    const placeholderRow = skeleton.lastElementChild!;
    expect(skeleton).toHaveAttribute("aria-hidden", "true");
    expect(placeholderRow.className).toContain("h-14");
    unmount();

    renderWithRouter(
      <AgentsPanel
        descendants={[working]}
        parentSessionId="p1"
        loading={false}
      />,
    );
    const row = (await screen.findByTestId("agents-panel")).querySelector(
      "li",
    )!;
    expect(row.className).toContain("h-14");
    expect(screen.queryByTestId("agents-panel-skeleton")).toBeNull();
  });
});
