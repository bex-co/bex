import { describe, it, expect } from "vitest";
import { isTakeoverOnlyConflict } from "@/features/blueprints/lib/takeover";

// w4/m125: `/blueprints/new` disables Deploy on any invalid preview, which is
// right for a manifest that does not parse and a dead end for a conflict — the
// confirmation phrase only arrives in the create's refusal, so a disabled
// button means reading about a takeover you can never perform. w5/m125: the
// preview's problems are told apart by code, not wording.
describe("isTakeoverOnlyConflict", () => {
  // bex-api's own preview entry (lego/backend/internal/apps/
  // blueprint_ownership.go blueprintOwnershipError).
  const resource = {
    code: "BLUEPRINT_RESOURCE_CONFLICT",
    error:
      'service "web" is managed by blueprint blp-2; retry with confirm="takeover blueprint blp-2" to transfer ownership to this blueprint',
  };
  const otherResource = {
    code: "BLUEPRINT_RESOURCE_CONFLICT",
    error:
      'database "db" is managed by blueprint blp-2; retry with confirm="takeover blueprint blp-2" to transfer ownership to this blueprint',
  };
  const invalid = { code: "", error: "services[0].name is required" };
  // A compiler problem carries a code too (blueprint_ir.go).
  const duplicate = {
    code: "BLUEPRINT_DUPLICATE_RESOURCE",
    error: 'duplicate name "web" for service (first declared at #/services/0)',
  };

  it("is true for a resource conflict", () => {
    expect(isTakeoverOnlyConflict([resource])).toBe(true);
  });

  it("is true when every problem is takeover-able", () => {
    expect(isTakeoverOnlyConflict([resource, otherResource])).toBe(true);
  });

  it("is false when any problem is not", () => {
    expect(isTakeoverOnlyConflict([resource, invalid])).toBe(false);
  });

  it("is false for an ordinary validation failure", () => {
    expect(isTakeoverOnlyConflict([invalid])).toBe(false);
  });

  it("is false for a coded problem no takeover resolves", () => {
    expect(isTakeoverOnlyConflict([duplicate])).toBe(false);
    expect(isTakeoverOnlyConflict([resource, duplicate])).toBe(false);
  });

  // A server reword cannot strand Deploy, and a problem that merely quotes the
  // phrase is not a conflict.
  it("reads the code, not the wording", () => {
    const reworded = { ...resource, error: "owned elsewhere" };
    const quoting = [
      { code: "", error: resource.error },
      { code: "", error: otherResource.error },
    ];
    expect(isTakeoverOnlyConflict([reworded])).toBe(true);
    expect(isTakeoverOnlyConflict(quoting)).toBe(false);
  });

  // A valid manifest has no errors; Deploy is enabled for the ordinary reason,
  // not because everything vacuously matched.
  it("is false for no errors at all", () => {
    expect(isTakeoverOnlyConflict([])).toBe(false);
  });
});
