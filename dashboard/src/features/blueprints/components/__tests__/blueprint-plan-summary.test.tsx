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

function action(
  operation: string,
  name: string,
  extra: Partial<NonNullable<BlueprintPreviewPlan["actions"]>[number]> = {},
) {
  return {
    operation,
    kind: "service",
    name,
    sourcePath: `services[${name}]`,
    resourceId: null,
    changedFields: [],
    message: null,
    ...extra,
  };
}

// w4/m138: the backend classifies each resource (create / update / noop /
// detach); the summary must show that, and count only real changes.
describe("BlueprintPlanSummary operations", () => {
  it("says there are no changes when every action is a no-op", () => {
    render(
      <BlueprintPlanSummary
        plan={{
          ...plan,
          totalActions: 1,
          actions: [
            action("noop", "static-site", { resourceId: "srv-static" }),
          ],
        }}
        pricing={null}
      />,
    );
    expect(
      screen.getByText(
        "Blueprint file parsed successfully — no changes. Every resource already matches this file.",
      ),
    ).toBeInTheDocument();
    expect(screen.queryByText(/resource.* to sync|will change/)).toBeNull();
    expect(screen.getByText("No change")).toBeInTheDocument();
    expect(screen.getByText("static-site")).toBeInTheDocument();
  });

  it("labels create, update (with changed field paths), and no-change distinctly, counting only changes", () => {
    render(
      <BlueprintPlanSummary
        plan={{
          ...plan,
          totalActions: 3,
          actions: [
            action("noop", "cache", { kind: "key_value" }),
            action("update", "api", {
              changedFields: [
                { path: "services[0].plan" },
                { path: "services[0].envVars" },
              ],
            }),
            action("create", "worker"),
          ],
        }}
        pricing={null}
      />,
    );
    expect(
      screen.getByText(
        "Blueprint file parsed successfully — 2 resources will change.",
      ),
    ).toBeInTheDocument();
    const rows = screen.getAllByRole("listitem");
    // Creates first, then updates, then what is already in place.
    expect(rows.map((row) => row.textContent)).toEqual([
      "CreateworkerService",
      "UpdateapiServiceChanges: services[0].plan, services[0].envVars",
      "No changecacheKey Value",
    ]);
  });

  it("says an env group re-applies its write-only values, and shows a refused action's reason", () => {
    render(
      <BlueprintPlanSummary
        plan={{
          ...plan,
          actions: [
            action("update", "shared", {
              kind: "env_var_group",
              changedFields: [{ path: "envVarGroups[0].envVars" }],
            }),
            action("error", "db", {
              kind: "postgres",
              message: "plan cannot shrink storage",
            }),
          ],
        }}
        pricing={null}
      />,
    );
    expect(
      screen.getByText(
        "Group values are write-only, so every declared variable is re-applied.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Can't apply")).toBeInTheDocument();
    expect(screen.getByText("plan cannot shrink storage")).toBeInTheDocument();
  });

  it("falls back to name groups and totalActions when the plan has no actions", () => {
    render(
      <BlueprintPlanSummary
        plan={{ ...plan, actions: null, totalActions: 1 }}
        pricing={null}
      />,
    );
    expect(
      screen.getByText(
        "Blueprint file parsed successfully — 1 resource will change.",
      ),
    ).toBeInTheDocument();
    expect(screen.getByText("Services:")).toBeInTheDocument();
  });
});
