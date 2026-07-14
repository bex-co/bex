import { describe, it, expect, vi, beforeEach } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { AssignServicesDialog } from "@/features/environments/components/assign-services-dialog";
import type { EnvironmentView } from "@/features/environments/hooks/use-environments";
import type { ServiceView } from "@/features/services/types";

const setServices = vi.fn();
vi.mock("@/features/environments/hooks/use-set-environment-services", () => ({
  useSetEnvironmentServices: () => ({ setServices, busyId: null }),
}));

function svc(id: string): ServiceView {
  return {
    id,
    name: id,
    type: "web_service",
    suspended: false,
    phase: "Running",
    url: null,
    createdAt: null,
    replicas: 1,
    revision: "r1",
    plan: null,
    idleTTLSeconds: null,
    schedule: null,
    command: null,
  } as ServiceView;
}

const env: EnvironmentView = {
  id: "env-1",
  projectId: "prj-1",
  name: "staging",
  ownerId: "tea-1",
  createdAt: null,
  serviceIds: ["api"],
};

beforeEach(() => {
  setServices.mockReset();
  setServices.mockResolvedValue(true);
});

describe("AssignServicesDialog", () => {
  it("pre-checks the environment's current services", () => {
    render(
      <AssignServicesDialog
        environment={env}
        services={[svc("api"), svc("web"), svc("worker")]}
        open
        onOpenChange={vi.fn()}
      />,
    );

    expect(screen.getByRole("checkbox", { name: /api/ })).toBeChecked();
    expect(screen.getByRole("checkbox", { name: /web/ })).not.toBeChecked();
    expect(screen.getByRole("checkbox", { name: /worker/ })).not.toBeChecked();
  });

  it("full-replaces membership: unchecking removes and checking assigns", async () => {
    const onOpenChange = vi.fn();
    const user = userEvent.setup();
    render(
      <AssignServicesDialog
        environment={env}
        services={[svc("api"), svc("web"), svc("worker")]}
        open
        onOpenChange={onOpenChange}
      />,
    );

    // Drop the current member and add two others.
    await user.click(screen.getByRole("checkbox", { name: /api/ }));
    await user.click(screen.getByRole("checkbox", { name: /web/ }));
    await user.click(screen.getByRole("checkbox", { name: /worker/ }));
    await user.click(screen.getByRole("button", { name: "Save" }));

    expect(setServices).toHaveBeenCalledTimes(1);
    const [id, name, ids] = setServices.mock.calls[0];
    expect(id).toBe("env-1");
    expect(name).toBe("staging");
    expect([...ids].sort()).toEqual(["web", "worker"]);
    // On success the dialog closes; the hook's refetchQueries refreshes the
    // Environments + Projects lists, so no refetch callback is threaded in.
    expect(onOpenChange).toHaveBeenCalledWith(false);
  });

  it("shows an empty state when the workspace has no services", () => {
    render(
      <AssignServicesDialog
        environment={env}
        services={[]}
        open
        onOpenChange={vi.fn()}
      />,
    );

    expect(
      screen.getByText("This workspace has no services to assign yet."),
    ).toBeInTheDocument();
    expect(screen.queryByRole("checkbox")).not.toBeInTheDocument();
  });
});
