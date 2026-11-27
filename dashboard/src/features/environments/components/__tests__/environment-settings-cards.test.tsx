import { beforeEach, describe, expect, it, vi } from "vitest";
import { render, screen, waitFor, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { CombinedGraphQLErrors } from "@apollo/client/errors";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";
import {
  EnvironmentConnectionsCard,
  EnvironmentDeleteCard,
  EnvironmentNameCard,
  EnvironmentPermissionsCard,
} from "@/features/environments/components/environment-settings-cards";
import { isolationGated } from "@/features/environments/lib/settings";

const navigate = vi.fn();
const invalidate = vi.fn();
vi.mock("@tanstack/react-router", async (importOriginal) => {
  const actual =
    await importOriginal<typeof import("@tanstack/react-router")>();
  const { PlainLink } = await import("@/test/mocks/router-link");
  return {
    ...actual,
    Link: PlainLink,
    useNavigate: () => navigate,
    useRouter: () => ({ invalidate }),
  };
});

const update = vi.fn();
vi.mock("@/features/environments/hooks/use-update-environment", () => ({
  useUpdateEnvironment: () => ({ update, saving: false }),
}));

const remove = vi.fn();
vi.mock("@/features/environments/hooks/use-delete-environment", () => ({
  useDeleteEnvironment: () => ({ remove, deleting: null }),
}));

const staging: EnvironmentView = {
  id: "evm-staging",
  projectId: "prj-shop",
  name: "staging",
  ownerId: "tea-1",
  createdAt: null,
  serviceIds: [],
  databaseIds: [],
  keyValueIds: [],
  envGroupIds: [],
  protectedStatus: "unprotected",
  networkIsolationEnabled: false,
  ipAllowListEntries: [],
};

function refusal(code: string, message: string) {
  return new CombinedGraphQLErrors({
    errors: [{ message, extensions: { code } }],
  });
}

beforeEach(() => {
  navigate.mockReset();
  invalidate.mockReset();
  update.mockReset();
  update.mockResolvedValue({ ok: true });
  remove.mockReset();
  remove.mockResolvedValue({ ok: true });
});

describe("EnvironmentNameCard (w1/m683)", () => {
  it("renames through the patch and leaves edit mode", async () => {
    const user = userEvent.setup();
    render(<EnvironmentNameCard environment={staging} />);
    const field = screen.getByRole("textbox", { name: "Environment name" });
    expect(field).toHaveAttribute("readonly");

    await user.click(screen.getByRole("button", { name: "Edit" }));
    expect(field).not.toHaveAttribute("readonly");
    expect(field).toHaveFocus();
    await user.clear(field);
    await user.type(field, "  qa  ");
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(update).toHaveBeenCalledWith("evm-staging", { name: "qa" });
    await waitFor(() =>
      expect(screen.getByRole("button", { name: "Edit" })).toBeInTheDocument(),
    );
    // The document title is the loader's, so the route re-reads it.
    expect(invalidate).toHaveBeenCalled();
  });

  it("refuses an empty name with Render's sentence and sends nothing", async () => {
    const user = userEvent.setup();
    render(<EnvironmentNameCard environment={staging} />);
    await user.click(screen.getByRole("button", { name: "Edit" }));
    await user.clear(screen.getByRole("textbox", { name: "Environment name" }));
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(screen.getByRole("alert")).toHaveTextContent(
      "Environment name is required",
    );
    expect(update).not.toHaveBeenCalled();
  });

  it("words bex-api's duplicate-name refusal as Render does, under the field", async () => {
    const user = userEvent.setup();
    update.mockResolvedValue({
      ok: false,
      error: refusal(
        "CONFLICT",
        'conflict: an environment named "production" already exists in this project',
      ),
    });
    render(<EnvironmentNameCard environment={staging} />);
    await user.click(screen.getByRole("button", { name: "Edit" }));
    const field = screen.getByRole("textbox", { name: "Environment name" });
    await user.clear(field);
    await user.type(field, "production{Enter}");

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Name is already used by another environment in this project",
    );
    expect(field).toHaveAttribute("aria-invalid", "true");
    // Still editing, with the draft kept.
    expect(field).toHaveValue("production");
  });

  it("shows any other refusal on the card, in the server's words", async () => {
    const user = userEvent.setup();
    update.mockResolvedValue({
      ok: false,
      error: refusal("FORBIDDEN", "forbidden: you may not rename this"),
    });
    render(<EnvironmentNameCard environment={staging} />);
    await user.click(screen.getByRole("button", { name: "Edit" }));
    await user.type(
      screen.getByRole("textbox", { name: "Environment name" }),
      "-2{Enter}",
    );
    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Forbidden: you may not rename this",
    );
  });

  it("keeps Save off until the name changes, and Cancel restores it", async () => {
    const user = userEvent.setup();
    render(<EnvironmentNameCard environment={staging} />);
    await user.click(screen.getByRole("button", { name: "Edit" }));
    expect(screen.getByRole("button", { name: "Save" })).toBeDisabled();
    const field = screen.getByRole("textbox", { name: "Environment name" });
    await user.type(field, "x");
    await user.click(screen.getByRole("button", { name: "Cancel" }));
    expect(field).toHaveValue("staging");
    expect(update).not.toHaveBeenCalled();
  });

  it("disables Edit with the role's reason for a member who may not rename", () => {
    render(
      <EnvironmentNameCard
        environment={staging}
        reason="Viewers cannot change resources."
      />,
    );
    expect(screen.getByRole("button", { name: "Edit" })).toBeDisabled();
  });
});

describe("EnvironmentPermissionsCard (w1/m683)", () => {
  it("protects an environment without asking", async () => {
    const user = userEvent.setup();
    render(<EnvironmentPermissionsCard environment={staging} />);
    const protectedOption = screen.getByRole("radio", { name: /Protected/ });
    expect(protectedOption).toBeDisabled();

    await user.click(screen.getByRole("button", { name: "Edit" }));
    await user.click(protectedOption);
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(screen.queryByRole("alertdialog")).toBeNull();
    expect(update).toHaveBeenCalledWith("evm-staging", {
      protectedStatus: "protected",
    });
  });

  it("asks Render's Remove restrictions before unprotecting one", async () => {
    const user = userEvent.setup();
    render(
      <EnvironmentPermissionsCard
        environment={{ ...staging, protectedStatus: "protected" }}
      />,
    );
    expect(screen.getByRole("radio", { name: /Protected/ })).toBeChecked();
    await user.click(screen.getByRole("button", { name: "Edit" }));
    await user.click(screen.getByRole("radio", { name: /Default/ }));
    await user.click(screen.getByRole("button", { name: "Save" }));

    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveTextContent(
      "Removing restrictions for the staging environment will allow all team members to perform destructive actions.",
    );
    expect(update).not.toHaveBeenCalled();
    await user.click(
      within(dialog).getByRole("button", { name: "Yes, remove restrictions" }),
    );
    expect(update).toHaveBeenCalledWith("evm-staging", {
      protectedStatus: "unprotected",
    });
  });

  it("keeps the card editing with the refusal when the save fails", async () => {
    const user = userEvent.setup();
    update.mockResolvedValue({
      ok: false,
      error: refusal("FORBIDDEN", "forbidden: only an admin may"),
    });
    render(<EnvironmentPermissionsCard environment={staging} />);
    await user.click(screen.getByRole("button", { name: "Edit" }));
    await user.click(screen.getByRole("radio", { name: /Protected/ }));
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(await screen.findByRole("alert")).toHaveTextContent(
      "Forbidden: only an admin may",
    );
    expect(screen.getByRole("button", { name: "Save" })).toBeInTheDocument();
  });
});

describe("EnvironmentConnectionsCard (w1/m683)", () => {
  it("blocks connections after Render's Restrict connections dialog", async () => {
    const user = userEvent.setup();
    render(
      <EnvironmentConnectionsCard
        environment={staging}
        projectName="shop"
        gated={false}
      />,
    );
    await user.click(screen.getByRole("button", { name: "Edit" }));
    await user.click(screen.getByRole("radio", { name: /Blocked/ }));
    await user.click(screen.getByRole("button", { name: "Save" }));

    const dialog = await screen.findByRole("alertdialog");
    expect(
      within(dialog).getByRole("heading", { name: "Restrict connections" }),
    ).toBeInTheDocument();
    expect(dialog).toHaveTextContent(
      "Confirm you want to isolate connections for shop / staging",
    );
    expect(dialog).toHaveTextContent(
      "Existing open network connections will not be affected.",
    );
    await user.click(
      within(dialog).getByRole("button", { name: "Restrict connections" }),
    );
    expect(update).toHaveBeenCalledWith("evm-staging", {
      networkIsolationEnabled: true,
    });
  });

  it("allows them again after Yes, remove restrictions", async () => {
    const user = userEvent.setup();
    render(
      <EnvironmentConnectionsCard
        environment={{ ...staging, networkIsolationEnabled: true }}
        projectName="shop"
        gated={false}
      />,
    );
    await user.click(screen.getByRole("button", { name: "Edit" }));
    await user.click(screen.getByRole("radio", { name: /Allowed/ }));
    await user.click(screen.getByRole("button", { name: "Save" }));
    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveTextContent("will no longer be blocked");
    await user.click(
      within(dialog).getByRole("button", { name: "Yes, remove restrictions" }),
    );
    expect(update).toHaveBeenCalledWith("evm-staging", {
      networkIsolationEnabled: false,
    });
  });

  it("is Render's plan gate below Pro: no Edit, and Upgrade", () => {
    render(
      <EnvironmentConnectionsCard
        environment={staging}
        projectName="shop"
        gated
      />,
    );
    expect(screen.queryByRole("button", { name: "Edit" })).toBeNull();
    expect(
      screen.getByText(
        "Block cross-environment connections by upgrading to the Pro plan.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "Upgrade" })).toHaveAttribute(
      "href",
      "/workspace/settings",
    );
  });

  it("gates only turning isolation on below Pro: an isolated environment can always turn it off", () => {
    expect(isolationGated("hobby", false)).toBe(true);
    expect(isolationGated("hobby", true)).toBe(false);
    expect(isolationGated("pro", false)).toBe(false);
    expect(isolationGated("scale", false)).toBe(false);
  });
});

describe("EnvironmentDeleteCard (w1/m683)", () => {
  it("deletes only after the sudo phrase, then lands on the project's settings", async () => {
    const user = userEvent.setup();
    render(
      <EnvironmentDeleteCard environment={staging} onlyEnvironment={false} />,
    );
    await user.click(
      screen.getByRole("button", { name: "Delete environment" }),
    );
    const dialog = await screen.findByRole("alertdialog");
    expect(dialog).toHaveTextContent(
      "This environment will be permanently deleted, along with all of its services.",
    );
    const confirm = within(dialog).getByRole("button", {
      name: "Delete environment",
    });
    expect(confirm).toBeDisabled();
    await user.type(
      within(dialog).getByRole("textbox"),
      "sudo delete environment stagin",
    );
    expect(confirm).toBeDisabled();
    await user.type(within(dialog).getByRole("textbox"), "g");
    await user.click(confirm);

    expect(remove).toHaveBeenCalledWith("evm-staging", "staging");
    await waitFor(() =>
      expect(navigate).toHaveBeenCalledWith(
        expect.objectContaining({
          to: "/project/$projectId/settings",
          params: { projectId: "prj-shop" },
        }),
      ),
    );
  });

  it("says its env groups stay when it holds some", async () => {
    const user = userEvent.setup();
    render(
      <EnvironmentDeleteCard
        environment={{ ...staging, envGroupIds: ["evg-1"] }}
        onlyEnvironment={false}
      />,
    );
    await user.click(
      screen.getByRole("button", { name: "Delete environment" }),
    );
    expect(await screen.findByRole("alertdialog")).toHaveTextContent(
      "Its environment groups are kept but become unassigned",
    );
  });

  it("keeps a refusal in the dialog and stays on the page", async () => {
    const user = userEvent.setup();
    remove.mockResolvedValue({ ok: false, message: "" });
    render(
      <EnvironmentDeleteCard environment={staging} onlyEnvironment={false} />,
    );
    await user.click(
      screen.getByRole("button", { name: "Delete environment" }),
    );
    const dialog = await screen.findByRole("alertdialog");
    await user.type(
      within(dialog).getByRole("textbox"),
      "sudo delete environment staging",
    );
    await user.click(
      within(dialog).getByRole("button", { name: "Delete environment" }),
    );
    expect(await within(dialog).findByRole("alert")).toHaveTextContent(
      "Couldn't delete environment. Please try again in a bit. Sorry!",
    );
    expect(navigate).not.toHaveBeenCalled();
  });

  it("refuses a project's only environment as Render does", () => {
    render(<EnvironmentDeleteCard environment={staging} onlyEnvironment />);
    expect(
      screen.getByText(
        /This environment cannot be deleted as projects require at least one environment/,
      ),
    ).toBeInTheDocument();
    expect(screen.getByRole("link", { name: "project" })).toHaveAttribute(
      "href",
      "/project/prj-shop/settings",
    );
    expect(
      screen.getByRole("button", { name: "Delete environment" }),
    ).toBeDisabled();
  });

  it("is Render's Environment Deletion card for a member who may not delete it", () => {
    render(
      <EnvironmentDeleteCard
        environment={staging}
        onlyEnvironment={false}
        deniedReason="Only an admin can delete a protected environment."
      />,
    );
    expect(
      screen.getByRole("heading", { name: "Environment Deletion" }),
    ).toBeInTheDocument();
    expect(
      screen.getByText("Only an admin can delete a protected environment."),
    ).toBeInTheDocument();
    expect(
      screen.queryByRole("button", { name: "Delete environment" }),
    ).toBeNull();
  });
});
