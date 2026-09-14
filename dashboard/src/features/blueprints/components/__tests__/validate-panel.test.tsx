import { describe, it, expect, vi, beforeEach } from "vitest";
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
    expect(
      screen.getByRole("button", { name: /run validate/i }),
    ).toBeInTheDocument();
    expect(screen.queryByText(/manifest is valid/i)).not.toBeInTheDocument();
    expect(screen.queryByText(/manifest has errors/i)).not.toBeInTheDocument();
  });

  it("shows a valid result after validate succeeds", async () => {
    validateState.validate = vi.fn(async () => {
      validateState.result = { valid: true, errors: [] };
      return validateState.result;
    });

    render(<ValidatePanel manifest="services:\n  - name: api" />);
    await userEvent.click(
      screen.getByRole("button", { name: /run validate/i }),
    );

    expect(await screen.findByText(/manifest is valid/i)).toBeInTheDocument();
  });

  it("shows every problem when validation returns several", async () => {
    validateState.validate = vi.fn(async () => {
      validateState.result = {
        valid: false,
        errors: [
          "at '/services/0/plan': additional properties 'plan' not allowed",
          "at '/services/1': additional properties 'totallyUnknownField' not allowed",
          "service \"qa-static-bp\": staticPublishPath is required for a static_site",
        ],
      };
      return validateState.result;
    });

    render(<ValidatePanel manifest="bad yaml" />);
    await userEvent.click(
      screen.getByRole("button", { name: /run validate/i }),
    );

    expect(await screen.findByText(/manifest has errors/i)).toBeInTheDocument();
    expect(
      screen.getByText("at '/services/0/plan': additional properties 'plan' not allowed"),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        "at '/services/1': additional properties 'totallyUnknownField' not allowed",
      ),
    ).toBeInTheDocument();
    expect(
      screen.getByText(
        'service "qa-static-bp": staticPublishPath is required for a static_site',
      ),
    ).toBeInTheDocument();
  });

  it("disables the button when manifest is empty", () => {
    render(<ValidatePanel manifest="" />);
    expect(
      screen.getByRole("button", { name: /run validate/i }),
    ).toBeDisabled();
  });
});
