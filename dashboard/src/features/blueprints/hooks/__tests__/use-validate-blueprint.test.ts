import { act, renderHook } from "@testing-library/react";
import { beforeEach, expect, it, vi } from "vitest";
import { ValidateBlueprintDocument } from "@/graphql/definitions";
import { useValidateBlueprint } from "../use-validate-blueprint";

const run = vi.fn();
const lazy = vi.fn();
vi.mock("@apollo/client/react", () => ({
  useLazyQuery: (...args: unknown[]) => lazy(...args),
}));

beforeEach(() => {
  run.mockReset();
  run.mockResolvedValue({
    data: {
      validateBlueprint: {
        valid: false,
        errors: ["refused"],
        errorDetails: [],
      },
    },
  });
  lazy.mockReset();
  lazy.mockReturnValue([run, { loading: false }]);
});

// Unscoped, the server validates in the caller's default workspace, which may
// not hold the Blueprint's services, so an existing-state refusal that a sync
// hits read as valid (w8/m64).
it("validates a Blueprint's manifest on behalf of that Blueprint", async () => {
  const { result } = renderHook(() => useValidateBlueprint("blp-1"));
  await act(async () => {
    await result.current.validate("services: []");
  });
  expect(lazy).toHaveBeenCalledWith(
    ValidateBlueprintDocument,
    expect.anything(),
  );
  expect(run).toHaveBeenCalledWith({
    variables: { bexYaml: "services: []", blueprintId: "blp-1" },
  });
  expect(result.current.result?.valid).toBe(false);
});
