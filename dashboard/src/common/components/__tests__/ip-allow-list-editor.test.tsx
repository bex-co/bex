import { describe, expect, it, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import { IPAllowListEditor } from "@/common/components/ip-allow-list-editor";

const labels = {
  hint: "Only these CIDRs may reach the endpoint.",
  open: "Open to all source IPs.",
  descriptionPlaceholder: "Description (optional)",
  cidr: "New CIDR block",
  cidrRule: (number: number) => `CIDR block for rule ${number}`,
  description: "New rule description",
  descriptionRule: (number: number) => `Description for rule ${number}`,
  add: "Add",
  save: "Save allowlist",
  invalid: "Enter a valid CIDR.",
  remove: (cidr: string) => `Remove ${cidr}`,
  moveUp: (cidr: string) => `Move up ${cidr}`,
  moveDown: (cidr: string) => `Move down ${cidr}`,
};

describe("IPAllowListEditor", () => {
  it("names add-row and existing-row inputs (not by placeholder example) (m101/t010)", () => {
    render(
      <IPAllowListEditor
        entries={[{ cidrBlock: "10.0.0.0/8", description: "corp" }]}
        labels={labels}
        saving={false}
        onSave={vi.fn(async () => true)}
      />,
    );

    expect(
      screen.getByRole("textbox", { name: "New CIDR block" }),
    ).toHaveAttribute("placeholder", "203.0.113.0/24");
    expect(
      screen.getByRole("textbox", { name: "New rule description" }),
    ).toBeInTheDocument();
    expect(
      screen.getByRole("textbox", { name: "CIDR block for rule 1" }),
    ).toHaveValue("10.0.0.0/8");
    expect(
      screen.getByRole("textbox", { name: "Description for rule 1" }),
    ).toHaveValue("corp");
    // Placeholder text alone must not be the accessible name.
    expect(
      screen.queryByRole("textbox", { name: "203.0.113.0/24" }),
    ).not.toBeInTheDocument();
  });
});
