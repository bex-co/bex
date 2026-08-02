import fs from "fs";
import path from "path";
import { Kind, parse, print } from "graphql";

describe("mobile usage glance document", () => {
  it("selects only read-only totals and explicit coverage evidence", () => {
    const source = fs.readFileSync(
      path.resolve(process.cwd(), "src/features/usage/api/usage.graphql"),
      "utf8",
    );
    const parsed = parse(source);
    const operation = parsed.definitions.find(
      (definition) => definition.kind === Kind.OPERATION_DEFINITION,
    );
    expect(
      String(
        operation?.kind === Kind.OPERATION_DEFINITION
          ? operation.operation
          : undefined,
      ),
    ).toBe("query");

    const document = print(parsed);
    for (const forbidden of [
      "billing",
      "invoice",
      "checkout",
      "portal",
      "tax",
      "plan",
      "estimatedCost",
      "mutation",
    ]) {
      expect(new RegExp(`\\b${forbidden}\\b`, "i").test(document)).toBe(false);
    }
    for (const required of [
      "period",
      "coverage",
      "state",
      "through",
      "degradedSources",
      "services",
      "rows",
      "kind",
      "total",
    ]) {
      expect(new RegExp(`\\b${required}\\b`).test(document)).toBe(true);
    }
  });
});
