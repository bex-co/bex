import { describe, it, expect, vi } from "vitest";
import { fireEvent, render, screen } from "@testing-library/react";
import { PlannedSyncPanel } from "../planned-sync-panel";
import type { PlannedSyncView } from "../../hooks/use-planned-sync";

const planned: PlannedSyncView = {
  id: "exe-1",
  commitId: "0123456789abcdef",
  actions: [
    {
      operation: "create",
      kind: "service",
      name: "express-hello-world",
      sourcePath: "#/services/0",
      resourceId: null,
      changedFields: [],
      message: null,
    },
  ],
  requiredManualEnvVars: [
    {
      serviceType: "web",
      serviceName: "express-hello-world",
      key: "API_TOKEN",
    },
  ],
};

function renderPanel(
  props: Partial<Parameters<typeof PlannedSyncPanel>[0]> = {},
) {
  const onApprove = vi.fn().mockResolvedValue(true);
  const onReplan = vi.fn().mockResolvedValue(undefined);
  render(
    <PlannedSyncPanel
      planned={planned}
      loading={false}
      busy={false}
      onApprove={onApprove}
      onReplan={onReplan}
      {...props}
    />,
  );
  return { onApprove, onReplan };
}

// w2/m256 t012: a created Blueprint applies only by approving its plan, and
// the plan's sync:false values are part of the approval.
describe("PlannedSyncPanel", () => {
  it("lists the planned changes", () => {
    renderPanel();
    expect(screen.getByText("express-hello-world")).toBeInTheDocument();
  });

  it("cannot approve until every required value is entered, then sends them", () => {
    const { onApprove } = renderPanel();
    const approve = screen.getByRole("button", { name: /approve/i });
    expect(approve).toBeDisabled();
    fireEvent.change(screen.getByLabelText(/express-hello-world.*API_TOKEN/), {
      target: { value: "s3cret" },
    });
    expect(approve).toBeEnabled();
    fireEvent.click(approve);
    expect(onApprove).toHaveBeenCalledWith({
      "express-hello-world/API_TOKEN": "s3cret",
    });
  });

  it("approves at once when the plan needs no values", () => {
    const { onApprove } = renderPanel({
      planned: { ...planned, requiredManualEnvVars: [] },
    });
    fireEvent.click(screen.getByRole("button", { name: /approve/i }));
    expect(onApprove).toHaveBeenCalledWith({});
  });

  it("says when a plan was stored without its changes, and still approves", () => {
    const { onApprove } = renderPanel({
      planned: { ...planned, actions: [], requiredManualEnvVars: [] },
    });
    expect(
      screen.getByText(/recorded before plans kept their changes/),
    ).toBeInTheDocument();
    fireEvent.click(screen.getByRole("button", { name: /approve/i }));
    expect(onApprove).toHaveBeenCalledWith({});
  });

  it("offers to plan again when nothing waits for approval", () => {
    const { onReplan } = renderPanel({ planned: null });
    expect(screen.queryByRole("button", { name: /approve/i })).toBeNull();
    fireEvent.click(screen.getByRole("button", { name: /plan again/i }));
    expect(onReplan).toHaveBeenCalled();
  });

  it("previews its ready geometry while loading", () => {
    const { container } = render(
      <PlannedSyncPanel
        planned={null}
        loading
        busy={false}
        onApprove={vi.fn()}
        onReplan={vi.fn()}
      />,
    );
    expect(
      container.querySelector('[data-skeleton-region="planned-sync"]'),
    ).not.toBeNull();
    expect(screen.queryByRole("button")).toBeNull();
  });
});
