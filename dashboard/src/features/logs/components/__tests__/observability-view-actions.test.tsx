import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ObservabilityViewActions } from "../observability-view-actions";
import type { SavedSearch } from "@/features/projects/hooks/use-saved-searches";
import { SavedSearchType } from "@/graphql/definitions";
import { useCapabilities } from "@/features/capabilities/hooks/use-capabilities";
import { mockCapabilities } from "@/test/mocks/capabilities";

// A view's header buttons (w3/m507): Edit view, Duplicate and the trash
// button, on every view, the defaults too. Duplicate opens the copy; Delete
// confirms with Render's sudo phrase and lands on the first remaining
// view, or the overview. A member who may not edit sees them disabled.

const mutations = {
  create: vi.fn(),
  update: vi.fn(),
  duplicate: vi.fn(),
  remove: vi.fn(),
  busy: null,
};
vi.mock("@/features/projects/hooks/use-saved-search-mutations", () => ({
  useSavedSearchMutations: () => mutations,
}));
const opened: unknown[] = [];
vi.mock("../../hooks/use-open-view", () => ({
  useOpenView: () => async (view: unknown) => {
    opened.push(view ?? null);
  },
}));
vi.mock("../edit-view-dialog", () => ({
  EditViewDialog: ({ target }: { target: { view: SavedSearch } | null }) =>
    target ? <div role="dialog">editing {target.view.name}</div> : null,
}));

const view = (
  id: string,
  name: string,
  type: SavedSearchType,
): SavedSearch => ({
  __typename: "SavedSearch",
  id,
  name,
  type,
  filters: { filters: [], version: 1 },
});
const APP = view(
  "svs-app",
  "Application Logs",
  SavedSearchType.ApplicationLogs,
);

function renderActions() {
  const onLeaving = vi.fn();
  render(
    <ObservabilityViewActions
      projectId="prj-1"
      view={APP}
      onLeaving={onLeaving}
    />,
  );
  return { onLeaving };
}

beforeEach(() => {
  opened.length = 0;
  mutations.duplicate.mockReset();
  mutations.remove.mockReset();
  vi.mocked(useCapabilities).mockReturnValue(mockCapabilities());
});

describe("ObservabilityViewActions", () => {
  it("opens Edit view on the view", async () => {
    const user = userEvent.setup();
    renderActions();
    await user.click(screen.getByRole("button", { name: "Edit view" }));
    expect(screen.getByRole("dialog")).toHaveTextContent(
      "editing Application Logs",
    );
  });

  it("duplicates the view and opens the copy", async () => {
    const user = userEvent.setup();
    const copy = view(
      "svs-copy",
      "Application Logs (copy)",
      SavedSearchType.ApplicationLogs,
    );
    mutations.duplicate.mockResolvedValue(copy);
    renderActions();
    await user.click(screen.getByRole("button", { name: "Duplicate" }));
    expect(mutations.duplicate).toHaveBeenCalledWith("svs-app");
    await waitFor(() => expect(opened).toEqual([copy]));
  });

  it("deletes the view behind Render's typed confirmation, then opens the first view left", async () => {
    const user = userEvent.setup();
    const metrics = view("svs-met", "Service Metrics", SavedSearchType.Metrics);
    const requests = view(
      "svs-req",
      "Request Logs",
      SavedSearchType.RequestLogs,
    );
    mutations.remove.mockImplementation(
      async (_id: string, onDeleted?: () => void) => {
        onDeleted?.();
        return [metrics, requests];
      },
    );
    const { onLeaving } = renderActions();

    await user.click(screen.getByRole("button", { name: "Delete view" }));
    const confirm = screen.getByRole("alertdialog");
    expect(confirm).toHaveTextContent("Delete Application Logs?");
    const go = within(confirm).getByRole("button", { name: "Delete view" });
    expect(go).toBeDisabled();
    await user.type(
      within(confirm).getByRole("textbox"),
      "sudo delete view Application Logs",
    );
    await user.click(go);

    expect(mutations.remove.mock.calls[0][0]).toBe("svs-app");
    // The page steps aside once bex-api has deleted the view, then takes
    // the next view's state back.
    expect(onLeaving.mock.calls[0]).toEqual(["svs-app"]);
    // The first remaining view opens, Service Metrics too (w3/m508).
    await waitFor(() => expect(opened).toEqual([metrics]));
    await waitFor(() => expect(onLeaving).toHaveBeenLastCalledWith(null));
  });

  it("lands on the project's overview when no view is left", async () => {
    const user = userEvent.setup();
    mutations.remove.mockResolvedValue([]);
    renderActions();
    await user.click(screen.getByRole("button", { name: "Delete view" }));
    const confirm = screen.getByRole("alertdialog");
    await user.type(
      within(confirm).getByRole("textbox"),
      "sudo delete view Application Logs",
    );
    await user.click(
      within(confirm).getByRole("button", { name: "Delete view" }),
    );
    await waitFor(() => expect(opened).toEqual([null]));
  });

  it("disables every button with the role's reason for a member who may not edit", async () => {
    const user = userEvent.setup();
    vi.mocked(useCapabilities).mockReturnValue(
      mockCapabilities({ role: "VIEWER", canCreate: false }),
    );
    renderActions();
    for (const name of ["Edit view", "Duplicate", "Delete view"]) {
      expect(screen.getByRole("button", { name })).toBeDisabled();
    }
    // The reason is keyboard-reachable: each disabled button sits in a
    // focusable wrapper whose tooltip names it.
    await user.tab();
    expect(
      await screen.findAllByText(
        "Your role can’t make this change. Ask a workspace admin.",
      ),
    ).not.toHaveLength(0);
    await user.click(screen.getByRole("button", { name: "Duplicate" }));
    expect(mutations.duplicate).not.toHaveBeenCalled();
  });
});
