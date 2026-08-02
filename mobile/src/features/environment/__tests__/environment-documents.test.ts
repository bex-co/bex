import fs from "fs";
import path from "path";
import { Kind, parse, visit } from "graphql";

const file = path.resolve(
  process.cwd(),
  "src/features/environment/api/environment.graphql",
);
const source = fs.readFileSync(file, "utf8");
const document = parse(source);

function operation(name: string) {
  const found = document.definitions.find(
    (definition) =>
      definition.kind === Kind.OPERATION_DEFINITION &&
      definition.name?.value === name,
  );
  if (!found || found.kind !== Kind.OPERATION_DEFINITION) {
    throw new Error(`operation ${name} not found`);
  }
  return found;
}

describe("mobile environment GraphQL inventory", () => {
  it("keeps the masked list structurally unable to fetch values", () => {
    const fields: string[] = [];
    visit(operation("MobileEnvVarKeys"), {
      Field(node) {
        fields.push(node.name.value);
      },
    });
    expect(fields).toEqual([
      "service",
      "id",
      "envVarKeys",
      "id",
      "key",
      "revision",
    ]);
    expect(fields.includes("value")).toBe(false);
  });

  it("reveals exactly one key and obtains the revision with its value", () => {
    const reveal = operation("MobileRevealEnvVar");
    expect(
      reveal.variableDefinitions?.map((item) => item.variable.name.value),
    ).toEqual(["serviceId", "key"]);
    const fields: string[] = [];
    visit(reveal, { Field: (node) => void fields.push(node.name.value) });
    expect(fields).toEqual([
      "service",
      "id",
      "envVar",
      "id",
      "key",
      "value",
      "revision",
    ]);
  });

  it("allows only a CAS update of one existing key", () => {
    const mutations = document.definitions.flatMap((definition) =>
      definition.kind === Kind.OPERATION_DEFINITION &&
      definition.operation === "mutation"
        ? [definition]
        : [],
    );
    expect(mutations.map((item) => item.name?.value)).toEqual([
      "MobilePatchSingleEnvVar",
    ]);
    expect(
      mutations[0]?.variableDefinitions?.map((item) => ({
        name: item.variable.name.value,
        required: item.type.kind === Kind.NON_NULL_TYPE,
      })),
    ).toEqual([
      { name: "serviceId", required: true },
      { name: "key", required: true },
      { name: "value", required: true },
      { name: "revision", required: true },
    ]);
    expect(source).toContain("expectedEnvRevision: $revision");
    expect(source).toContain("envVars: [{ key: $key, value: $value }]");
    expect(source).toContain('saveMode: "deploy"');
    const responseFields: string[] = [];
    visit(mutations[0]!, {
      Field: (node) => void responseFields.push(node.name.value),
    });
    expect(responseFields).toEqual([
      "patchServiceEnvironment",
      "envVarKeys",
      "rolledOut",
    ]);
    expect(responseFields.includes("value")).toBe(false);
    for (const forbidden of [
      "deleteEnvVar",
      "setEnvVars",
      "generateValue",
      "secretFiles",
      "fromKey",
      "delete:",
    ]) {
      expect(source.includes(forbidden)).toBe(false);
    }
  });
});
