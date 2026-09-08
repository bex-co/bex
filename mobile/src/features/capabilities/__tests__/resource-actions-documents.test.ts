import { readFileSync } from "node:fs";
import { join } from "node:path";
import { Kind, parse, visit } from "graphql";

const source = readFileSync(
  join(__dirname, "../api/resource-actions.graphql"),
  "utf8",
);

function selectedFields(operation: string): string[] {
  const fields: string[] = [];
  for (const definition of parse(source).definitions) {
    if (
      definition.kind === Kind.OPERATION_DEFINITION &&
      definition.name?.value === operation
    ) {
      visit(definition, {
        Field: (node) => void fields.push(node.name.value),
      });
    }
  }
  return [...new Set(fields)].sort();
}

describe("mobile resource-action GraphQL documents (w6/m141/t001)", () => {
  it("exposes the four projections with the minimum decision fields", () => {
    expect(source.includes("query MobileServerActions($id: String!)")).toBe(
      true,
    );
    expect(
      source.includes("query MobileDeployActions($serviceId: String!)"),
    ).toBe(true);
    expect(source.includes("query MobileDatabaseActions($id: String!)")).toBe(
      true,
    );
    expect(source.includes("query MobileKeyValueActions($id: String!)")).toBe(
      true,
    );
    const cases: [string, string][] = [
      ["MobileServerActions", "serverActions"],
      ["MobileDeployActions", "deployActions"],
      ["MobileDatabaseActions", "databaseActions"],
      ["MobileKeyValueActions", "keyValueActions"],
    ];
    for (const [operation, root] of cases) {
      expect(selectedFields(operation)).toEqual(
        ["action", "outcome", "precondition", "reason", root].sort(),
      );
    }
  });

  it("selects no sensitive or configuration fields", () => {
    const selected = new Set(
      [
        "MobileServerActions",
        "MobileDeployActions",
        "MobileDatabaseActions",
        "MobileKeyValueActions",
      ].flatMap(selectedFields),
    );
    for (const forbidden of [
      "connectionString",
      "connectionInfo",
      "password",
      "envVar",
      "confirm",
      "scope",
      "billing",
      "secret",
    ]) {
      expect(selected.has(forbidden)).toBe(false);
    }
  });
});
