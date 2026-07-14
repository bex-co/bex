import { describe, it, expect, vi } from "vitest";
import { render, screen } from "@testing-library/react";
import userEvent from "@testing-library/user-event";
import { ValidatePanel } from "../validate-panel";
import type { BlueprintValidationResult } from "@/features/blueprints/types";

const validateState: {
  validate: (yaml: string) => Promise<BlueprintValidationResult | null>;
  result: BlueprintValidationResult | null;
  loading: boolean;
} = {
  validate: vi.fn(async () => null),
  result: null,
  loading: false,
};
vi.mock("@/features/blueprints/hooks/use-validate-blueprint", () => ({
  useValidateBlueprint: () => validateState,
}));

beforeEach(() => {
  validateState.validate = vi.fn(async () => null);
  validateState.result = null;
  validateState.loading = false;
});

describe("ValidatePanel", () => {
  it("shows the validate button and no result before running", () => {
    render(<ValidatePanel manifest="services:\n  - name: api" />);
    expect(screen.getByRole("button", { name: /run validate/i })).toBeInTheDocument();
    expect(screen.queryByText(/manifest is valid/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/manifest has errors/i)).not.toBeInTheDocument();
  });

  it("shows a valid result after validate succeeds", async () => {
    validateState.validate = vi.fn(async () => {
      validateState.result = { valid: true, errors: [] };
      return validateState.result;
    });

    render(<ValidatePanel manifest="services:\n  - name: api" />);
    await userEvent.click(screen.getByRole("button", { name: /run validate/i }));

    expect(await screen.findByText(/manifest is valid/i)).toBeInTheDocument();
  });

  it("shows per-entry errors when validation fails", async () => {
    validateState.validate = vi.fn(async () => {
      validateState.result = {
        valid: false,
        errors: ["missing required field: name", "unknown field: fromService.envVarKey"],
      };
      return validateState.result;
    });

    render(<ValidatePanel manifest="bad yaml" />);
    await userEvent.click(screen.getByRole("button", { name: /run validate/i }));

    expect(await screen.findByText(/manifest has errors/i)).toBeInTheDocument();
    expect(screen.getByText("missing required field: name")).toBeInTheDocument();
    expect(screen.getByText("unknown field: fromService.envVarKey")).toBeInTheDocument();
  });

  it("disables the button when manifest is empty", () => {
    render(<ValidatePanel manifest="" />);
    expect(screen.getByRole("button", { name: /run validate/i })).toBeDisabled();
  });
});
