import { describe, expect, it } from "vitest";
import { render, screen } from "@testing-library/react";
import { ComputePlanList, type ComputePlanTier } from "../compute-plan-list";

// w6/m483: a service's create-form cards name each paid plan by Render's plan
// ID beside its legacy name, as the Compute page's cards do. Free (whose ID is
// its name) and a family without plan IDs (Key Value, Postgres) keep the name.
const tiers: ComputePlanTier[] = [
  { id: "free", planId: "free", name: "Free", cpu: "100m", memory: "512Mi" },
  {
    id: "starter",
    planId: "0.5c-512mb",
    name: "Starter",
    cpu: "500m",
    memory: "512Mi",
  },
  { id: "pro_plus", name: "Pro Plus", cpu: "4", memory: "10Gi" },
];

describe("ComputePlanList plan IDs (w6/m483)", () => {
  it("shows a plan ID beside the legacy name only where the tier has one", () => {
    render(
      <ComputePlanList
        instanceTypes={tiers}
        value="starter"
        ariaLabel="Compute Plan"
        onChange={() => {}}
      />,
    );

    const starter = screen.getByRole("radio", { name: /Starter/ });
    expect(starter).toHaveAccessibleName(/^0\.5c-512mb Starter/);
    expect(starter.querySelector(".font-mono")).toHaveTextContent("0.5c-512mb");
    for (const name of [/^Free/, /^Pro Plus/]) {
      expect(
        screen.getByRole("radio", { name }).querySelector(".font-mono"),
      ).toBeNull();
    }
  });
});
