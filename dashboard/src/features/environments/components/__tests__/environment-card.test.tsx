import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { EnvironmentCard } from "@/features/environments/components/environment-card";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";

const rename = vi.fn();
vi.mock("@/features/environments/hooks/use-rename-environment", () => ({
  useRenameEnvironment: () => ({ rename, busy: false }),
}));

const remove = vi.fn();
vi.mock("@/features/environments/hooks/use-delete-environment", () => ({
  useDeleteEnvironment: () => ({ remove, deleting: null }),
}));

// The assign-services dialog reaches Apollo when opened; the card never opens it
// in these tests, but stub it so mounting the (closed) dialog stays inert.
vi.mock("@/features/environments/components/assign-services-dialog", () => ({
  AssignServicesDialog: () => null,
}));

// An environment with no resolvable services renders the empty state instead of
// the shared ResourceTable (which pulls in service row-actions + Apollo).
const env: EnvironmentView = {
  id: "env-1",
  projectId: "prj-1",
  name: "staging",
  ownerId: "tea-1",
  createdAt: null,
  serviceIds: [],
};

function renderCard() {
  return render(
    <EnvironmentCard
      environment={env}
      services={[]}
      servicePending={null}
      onRunServiceAction={vi.fn()}
    />,
  );
}

beforeEach(() => {
  rename.mockReset();
  rename.mockResolvedValue(true);
  remove.mockReset();
  remove.mockResolvedValue(true);
});

describe("EnvironmentCard", () => {
  it("shows the environment name and its empty-services state", () => {
    renderCard();
    expect(screen.getByText("staging")).toBeInTheDocument();
    expect(
      screen.getByText("No services in this environment yet."),
    ).toBeInTheDocument();
  });

  it("renames via the overflow menu", async () => {
    const user = userEvent.setup();
    renderCard();

    await user.click(screen.getByRole("button", { name: "More actions" }));
    await user.click(await screen.findByRole("menuitem", { name: "Rename" }));

    const dialog = await screen.findByRole("dialog");
    const input = within(dialog).getByRole("textbox");
    await user.clear(input);
    await user.type(input, "production");
    await user.click(within(dialog).getByRole("button", { name: "Save" }));

    expect(rename).toHaveBeenCalledWith("env-1", "production");
  });

  it("deletes behind a confirmation", async () => {
    const user = userEvent.setup();
    renderCard();

    await user.click(screen.getByRole("button", { name: "More actions" }));
    await user.click(await screen.findByRole("menuitem", { name: "Delete" }));

    const dialog = await screen.findByRole("alertdialog");
    expect(
      within(dialog).getByText('Delete environment "staging"?'),
    ).toBeInTheDocument();
    await user.click(within(dialog).getByRole("button", { name: "Delete" }));

    expect(remove).toHaveBeenCalledWith("env-1", "staging");
  });
});
