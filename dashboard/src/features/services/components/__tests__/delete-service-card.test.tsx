import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen, within } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { DeleteServiceCard } from "@/features/services/components/delete-service-card";
import type { ServiceView } from "@/features/services/types";

const mockNavigate = vi.fn();
vi.mock("@tanstack/react-router", () => ({
  useNavigate: () => mockNavigate,
}));

const remove = vi.fn();
vi.mock("@/features/services/hooks/use-delete-service", () => ({
  useDeleteService: () => ({ remove, deleting: false }),
}));

function svc(overrides: Partial<ServiceView> = {}): ServiceView {
  return {
    id: "web",
    name: "web",
    slug: null,
    type: "web_service",
    suspended: false,
    phase: "Running",
    url: "https://web.onbex.co",
    internalAddress: null,
    createdAt: null,
    sshAddress: null,
    replicas: 1,
    revision: "r1",
    plan: "starter",
    idleTTLSeconds: 0,
    schedule: null,
    command: null,
    runs: [],
    repo: null,
    branch: null,
    rootDir: null,
    runtime: null,
    builder: null,
    buildCommand: null,
    startCommand: null,
    dockerfilePath: null,
    registryCredentialId: null,
    buildFilter: null,
    autoDeploy: null,
    notifyOnFail: null,
    notificationsToSend: null,
    healthCheckPath: null,
    maxShutdownDelaySeconds: null,
    preDeployCommand: null,
    renderSubdomainPolicy: null,
    publishPath: null,
    routes: [],
    headers: [],
    ipAllowList: null,
    ipAllowListEntries: null,
    maintenanceMode: null,
    ...overrides,
  };
}

// Render's live guard: the full "sudo delete <type words> <name>" phrase
// (docs/render-artifacts/protected-environments.md), typed into an input
// labeled "Sudo Command" — the prompt itself is body copy, not the label.
const PHRASE = "sudo delete web service web";

beforeEach(() => {
  mockNavigate.mockReset();
  remove.mockReset();
  remove.mockResolvedValue({ status: "success" });
});

describe("DeleteServiceCard — sudo type-to-confirm danger zone (w5/m14)", () => {
  it("opens the confirm dialog and keeps confirm disabled until the sudo phrase matches", async () => {
    const user = userEvent.setup();
    render(<DeleteServiceCard service={svc()} />);

    await user.click(screen.getByRole("button", { name: "Delete Service" }));

    const dialog = await screen.findByRole("dialog");
    const confirm = within(dialog).getByRole("button", {
      name: "Delete Service",
    });
    expect(confirm).toBeDisabled(); // nothing typed yet

    // The exact phrase is shown as body copy, not as the input's label.
    expect(within(dialog).getByText(PHRASE)).toBeInTheDocument();

    const input = within(dialog).getByLabelText("Sudo Command");
    await user.type(input, PHRASE.slice(0, -1));
    expect(confirm).toBeDisabled();
    expect(remove).not.toHaveBeenCalled();

    await user.type(input, PHRASE.slice(-1));
    expect(confirm).toBeEnabled();
  });

  it("a typo is a no-op, not a destroyed service", async () => {
    const user = userEvent.setup();
    render(<DeleteServiceCard service={svc()} />);

    await user.click(screen.getByRole("button", { name: "Delete Service" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(
      within(dialog).getByLabelText("Sudo Command"),
      `${PHRASE} `,
    );

    const confirm = within(dialog).getByRole("button", {
      name: "Delete Service",
    });
    expect(confirm).toBeDisabled();
    await user.click(confirm);
    expect(remove).not.toHaveBeenCalled();
  });

  it("on an exact match, deletes and redirects to the services list", async () => {
    const user = userEvent.setup();
    render(<DeleteServiceCard service={svc()} />);

    await user.click(screen.getByRole("button", { name: "Delete Service" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Sudo Command"), PHRASE);
    await user.click(
      within(dialog).getByRole("button", { name: "Delete Service" }),
    );

    expect(remove).toHaveBeenCalledWith("web", "web", undefined);
    expect(mockNavigate).toHaveBeenCalledWith({ to: "/", replace: true });
  });

  it("builds the phrase from Render's type words and the service name (bare name is not enough)", async () => {
    const user = userEvent.setup();
    render(
      <DeleteServiceCard
        service={svc({ id: "stable-id", name: "Customer API" })}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Delete Service" }));
    const dialog = await screen.findByRole("dialog");
    const input = within(dialog).getByLabelText("Sudo Command");
    const confirm = within(dialog).getByRole("button", {
      name: "Delete Service",
    });
    await user.type(input, "Customer API");
    expect(confirm).toBeDisabled();
    await user.clear(input);
    await user.type(input, "sudo delete web service Customer API");
    await user.click(confirm);

    expect(remove).toHaveBeenCalledWith("stable-id", "Customer API", undefined);
  });

  it("names non-web types with their own Render type words", async () => {
    const user = userEvent.setup();
    render(
      <DeleteServiceCard
        service={svc({ id: "rep", name: "reporter", type: "cron_job" })}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Delete Service" }));
    const dialog = await screen.findByRole("dialog");
    expect(
      within(dialog).getByText("sudo delete cron job reporter"),
    ).toBeInTheDocument();
  });

  it("stays put when the delete fails (no redirect)", async () => {
    remove.mockResolvedValue({ status: "error" });
    const user = userEvent.setup();
    render(<DeleteServiceCard service={svc()} />);

    await user.click(screen.getByRole("button", { name: "Delete Service" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Sudo Command"), PHRASE);
    await user.click(
      within(dialog).getByRole("button", { name: "Delete Service" }),
    );

    expect(remove).toHaveBeenCalledWith("web", "web", undefined);
    expect(mockNavigate).not.toHaveBeenCalled();
  });

  it("requires the backend's exact protected-environment phrase before retrying", async () => {
    remove
      .mockResolvedValueOnce({
        status: "confirmation_required",
        confirmation: "sudo delete service web",
      })
      .mockResolvedValueOnce({ status: "success" });
    const user = userEvent.setup();
    render(<DeleteServiceCard service={svc()} />);

    await user.click(screen.getByRole("button", { name: "Delete Service" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(within(dialog).getByLabelText("Sudo Command"), PHRASE);
    await user.click(
      within(dialog).getByRole("button", { name: "Delete Service" }),
    );

    expect(remove).toHaveBeenNthCalledWith(1, "web", "web", undefined);
    expect(mockNavigate).not.toHaveBeenCalled();

    // The dialog now shows the backend's authoritative phrase as body copy.
    expect(
      within(dialog).getByText("sudo delete service web"),
    ).toBeInTheDocument();
    const protectedInput = within(dialog).getByLabelText("Sudo Command");
    const confirm = within(dialog).getByRole("button", {
      name: "Delete Service",
    });
    await user.type(protectedInput, "sudo delete service we");
    expect(confirm).toBeDisabled();
    await user.type(protectedInput, "b");
    expect(confirm).toBeEnabled();
    await user.click(confirm);

    expect(remove).toHaveBeenNthCalledWith(
      2,
      "web",
      "web",
      "sudo delete service web",
    );
    expect(mockNavigate).toHaveBeenCalledWith({ to: "/", replace: true });
  });
});
