import { print } from "graphql";
import { describe, expect, it } from "vitest";
import { RollbackServiceDocument } from "@/graphql/definitions";

// w1/m152 t009: Render turns auto-deploy off for a rollback started in its
// dashboard and leaves it on for an API rollback. bex-api defaults to the API
// behaviour, so the dashboard's own document is the only thing that makes a
// dashboard rollback differ. Dropping the argument would silently let the next
// push undo every dashboard rollback.
describe("RollbackService document", () => {
  it("asks bex-api to turn auto-deploy off", () => {
    expect(print(RollbackServiceDocument)).toMatch(
      /rollbackService\([^)]*disableAutoDeploy:\s*true/,
    );
  });
});
