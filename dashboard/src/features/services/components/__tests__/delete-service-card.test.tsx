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
    type: "web_service",
    suspended: false,
    phase: "Running",
    url: "https://web.onbex.co",
    createdAt: null,
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
    ...overrides,
  };
}

beforeEach(() => {
  mockNavigate.mockReset();
  remove.mockReset();
  remove.mockResolvedValue(true);
});

describe("DeleteServiceCard — type-to-confirm danger zone (w5/m14)", () => {
  it("opens the confirm dialog and keeps confirm disabled until the name matches", async () => {
    const user = userEvent.setup();
    render(<DeleteServiceCard service={svc()} />);

    await user.click(screen.getByRole("button", { name: "Delete Service" }));

    const dialog = await screen.findByRole("dialog");
    const confirm = within(dialog).getByRole("button", {
      name: "Delete Service",
    });
    expect(confirm).toBeDisabled(); // nothing typed yet

    const input = within(dialog).getByLabelText(/Type web to confirm/);
    await user.type(input, "we");
    expect(confirm).toBeDisabled();
    expect(remove).not.toHaveBeenCalled();

    await user.type(input, "b");
    expect(confirm).toBeEnabled();
  });

  it("a typo is a no-op, not a destroyed service", async () => {
    const user = userEvent.setup();
    render(<DeleteServiceCard service={svc()} />);

    await user.click(screen.getByRole("button", { name: "Delete Service" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(
      within(dialog).getByLabelText(/Type web to confirm/),
      "web ",
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
    await user.type(
      within(dialog).getByLabelText(/Type web to confirm/),
      "web",
    );
    await user.click(
      within(dialog).getByRole("button", { name: "Delete Service" }),
    );

    expect(remove).toHaveBeenCalledWith("web", "web");
    expect(mockNavigate).toHaveBeenCalledWith({ to: "/" });
  });

  it("requires the immutable id when the human display name differs", async () => {
    const user = userEvent.setup();
    render(
      <DeleteServiceCard
        service={svc({ id: "stable-id", name: "Customer API" })}
      />,
    );

    await user.click(screen.getByRole("button", { name: "Delete Service" }));
    const dialog = await screen.findByRole("dialog");
    const input = within(dialog).getByLabelText(/Type stable-id to confirm/);
    const confirm = within(dialog).getByRole("button", {
      name: "Delete Service",
    });
    await user.type(input, "Customer API");
    expect(confirm).toBeDisabled();
    await user.clear(input);
    await user.type(input, "stable-id");
    await user.click(confirm);

    expect(remove).toHaveBeenCalledWith("stable-id", "Customer API");
  });

  it("stays put when the delete fails (no redirect)", async () => {
    remove.mockResolvedValue(false);
    const user = userEvent.setup();
    render(<DeleteServiceCard service={svc()} />);

    await user.click(screen.getByRole("button", { name: "Delete Service" }));
    const dialog = await screen.findByRole("dialog");
    await user.type(
      within(dialog).getByLabelText(/Type web to confirm/),
      "web",
    );
    await user.click(
      within(dialog).getByRole("button", { name: "Delete Service" }),
    );

    expect(remove).toHaveBeenCalledWith("web", "web");
    expect(mockNavigate).not.toHaveBeenCalled();
  });
});
