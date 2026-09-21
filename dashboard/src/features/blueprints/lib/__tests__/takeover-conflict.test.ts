import { describe, it, expect } from "vitest";
import { isTakeoverOnlyConflict } from "@/features/blueprints/lib/views";

// w4/m125: `/blueprints/new` disables Deploy on any invalid preview, which is
// right for a manifest that does not parse and a dead end for a conflict — the
// confirmation phrase only arrives in the create's refusal, so a disabled
// button means reading about a takeover you can never perform.
describe("isTakeoverOnlyConflict", () => {
  const connection =
    'blueprint blp-1 ("bpA") already tracks https://github.com/o/r@main from "a.yaml"; update it with updateBlueprint to change its path, or retry with confirm="takeover blueprint blp-1" to replace it';
  const resource =
    'service "web" is managed by blueprint blp-2; retry with confirm="takeover blueprint blp-2" to transfer ownership to this blueprint';

  it("is true for a connection conflict", () => {
    expect(isTakeoverOnlyConflict([connection])).toBe(true);
  });

  it("is true for a resource conflict", () => {
    expect(isTakeoverOnlyConflict([resource])).toBe(true);
  });

  it("is true when every problem is takeover-able", () => {
    expect(isTakeoverOnlyConflict([connection, resource])).toBe(true);
  });

  it("is false when any problem is not", () => {
    expect(
      isTakeoverOnlyConflict([connection, "services[0].name is required"]),
    ).toBe(false);
  });

  it("is false for an ordinary validation failure", () => {
    expect(isTakeoverOnlyConflict(["services[0].name is required"])).toBe(
      false,
    );
  });

  // A valid manifest has no errors; Deploy is enabled for the ordinary reason,
  // not because everything vacuously matched.
  it("is false for no errors at all", () => {
    expect(isTakeoverOnlyConflict([])).toBe(false);
  });
});
