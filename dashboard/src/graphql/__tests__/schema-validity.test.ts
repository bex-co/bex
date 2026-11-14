import { describe, expect, it } from "vitest";
import { readdirSync, readFileSync } from "node:fs";
import { join } from "node:path";
import {
  buildSchema,
  concatAST,
  parse,
  Source,
  validate,
  type DocumentNode,
  type GraphQLSchema,
} from "graphql";
import { readRepoText } from "@/test/go-source";

/**
 * Every GraphQL document the dashboard sends is valid against the schema
 * bex-api serves.
 *
 * `definitions.ts` is partly hand-maintained (dashboard/AGENTS.md calls the
 * hand-splice the common case), and nothing else compared an operation with the
 * server's schema, so a query for a field or an argument the server lacks failed
 * only in production (w1/116's `Cannot query field "undeployedChanges"`, from a
 * deploy skew). The schema is the Go test's golden SDL, which fails when the
 * served schema changes without it, so a backend change and the dashboard check
 * meet in one commit. After an intended change to the API, regenerate it:
 *
 *   UPDATE_SCHEMA_GOLDEN=1 go test ./internal/api -run TestGraphQLSchemaMatchesTheGolden
 *
 * (from lego/backend). Documents are the `src/**\/*.graphql` files codegen reads,
 * merged into one document because fragments are shared across files.
 */
const SCHEMA_PATH = "lego/backend/internal/api/testdata/schema.graphql";
const SRC_DIR = join(import.meta.dirname, "..", "..");

const schema = buildSchema(readRepoText(SCHEMA_PATH));

function documentFiles(): string[] {
  return readdirSync(SRC_DIR, { recursive: true })
    .map(String)
    .filter((file) => file.endsWith(".graphql"))
    .sort();
}

function parseFile(file: string): DocumentNode {
  return parse(new Source(readFileSync(join(SRC_DIR, file), "utf8"), file));
}

/** Each validation error as `file:line: message`. */
function problems(against: GraphQLSchema, documents: DocumentNode[]): string[] {
  return validate(against, concatAST(documents)).map((error) => {
    const location = error.nodes?.[0]?.loc;
    const where = location
      ? `${location.source.name}:${location.source.locationOffset.line + lineOf(location.source.body, location.start)}`
      : "(no location)";
    return `${where}: ${error.message}`;
  });
}

function lineOf(body: string, offset: number): number {
  return body.slice(0, offset).split("\n").length - 1;
}

function documentOf(text: string): DocumentNode {
  return parse(new Source(text, "probe.graphql"));
}

describe("the dashboard's GraphQL documents", () => {
  const files = documentFiles();

  it("are read from the files codegen reads, and from the golden schema", () => {
    expect(files.length).toBeGreaterThan(50);
    expect(
      Object.keys(schema.getQueryType()?.getFields() ?? {}).length,
    ).toBeGreaterThan(50);
    expect(schema.getMutationType()).toBeDefined();
  });

  it("are valid against the schema bex-api serves", () => {
    expect(problems(schema, files.map(parseFile))).toEqual([]);
  });
});

// The check is only worth having if it fails on what it exists to catch.
describe("the schema check", () => {
  it("refuses a field the server lacks", () => {
    expect(
      problems(schema, [documentOf("query { noSuchField { id } }")]),
    ).toEqual([expect.stringContaining('Cannot query field "noSuchField"')]);
  });

  it("refuses an argument the server lacks", () => {
    const query = schema.getQueryType()!;
    const [name] = Object.keys(query.getFields());
    expect(
      problems(schema, [documentOf(`query { ${name}(noSuchArg: 1) }`)]),
    ).toEqual(
      expect.arrayContaining([
        expect.stringContaining('Unknown argument "noSuchArg"'),
      ]),
    );
  });

  it("refuses a missing required argument", () => {
    const field = Object.values(schema.getQueryType()!.getFields()).find((f) =>
      f.args.some(
        (a) => a.type.toString().endsWith("!") && a.defaultValue === undefined,
      ),
    )!;
    expect(
      problems(schema, [documentOf(`query { ${field.name} { __typename } }`)]),
    ).toEqual(
      expect.arrayContaining([
        expect.stringContaining("is required, but it was not provided"),
      ]),
    );
  });

  it("refuses a fragment on a type the server lacks", () => {
    expect(
      problems(schema, [
        documentOf("fragment F on NoSuchType { id }\nquery { __typename }"),
      ]),
    ).toEqual(
      expect.arrayContaining([
        expect.stringContaining('Unknown type "NoSuchType"'),
      ]),
    );
  });

  it("names the file and line of a refused document", () => {
    const bad = parse(
      new Source(
        "\n\nquery Probe { noSuchField }",
        "feature/api/probe.graphql",
      ),
    );
    expect(problems(schema, [bad])).toEqual([
      expect.stringMatching(
        /^feature\/api\/probe\.graphql:3: Cannot query field/,
      ),
    ]);
  });
});
