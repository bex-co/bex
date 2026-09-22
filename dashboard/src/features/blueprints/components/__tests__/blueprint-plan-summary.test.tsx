import { describe, it, expect } from "vitest";
import { render, screen } from "@testing-library/react";
import { BlueprintPlanSummary } from "../blueprint-plan-summary";
import type { BlueprintPreviewPlan } from "../../types";

const plan: BlueprintPreviewPlan = {
  mode: "state_aware",
  services: ["new-api"],
  databases: [],
  keyValue: [],
  envGroups: [],
  syncFalseVars: [],
  totalActions: 2,
  actions: [
    {
      operation: "create",
      kind: "service",
      name: "new-api",
      sourcePath: "services[0]",
      resourceId: null,
      changedFields: [],
      message: null,
    },
    {
      operation: "detach",
      kind: "service",
      name: "old-api",
      sourcePath: "",
      resourceId: "srv-old",
      changedFields: [],
      message: null,
    },
  ],
};

describe("BlueprintPlanSummary detachment", () => {
  it("shows both the new service and the running resource leaving management", () => {
    render(<BlueprintPlanSummary plan={plan} pricing={null} />);
    expect(screen.getByText("new-api")).toBeInTheDocument();
    expect(screen.getByText("old-api")).toBeInTheDocument();
    expect(screen.getByText("srv-old")).toBeInTheDocument();
    expect(
      screen.getByText("Resources will stop being managed"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "These resources remain running and may continue to incur charges. This Blueprint will no longer manage them.",
      ),
    ).toBeInTheDocument();
  });
  it("leaves create-only previews without a detachment warning", () => {
    render(
      <BlueprintPlanSummary
        plan={{
          ...plan,
          actions: plan.actions!.filter(
            (action) => action.operation !== "detach",
          ),
        }}
        pricing={null}
      />,
    );
    expect(screen.getByText("new-api")).toBeInTheDocument();
    expect(
      screen.queryByText("Resources will stop being managed"),
    ).not.toBeInTheDocument();
  });
});
