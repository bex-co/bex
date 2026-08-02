import fs from "fs";
import path from "path";
import { Kind, parse, visit } from "graphql";

const source = fs.readFileSync(
  path.resolve(process.cwd(), "src/features/cron/api/cron-runs.graphql"),
  "utf8",
);
const document = parse(source);

describe("mobile cron GraphQL inventory", () => {
  it("selects only opaque run identity, status, and timestamps", () => {
    const fields: string[] = [];
    visit(document, { Field: (node) => void fields.push(node.name.value) });
    expect([...new Set(fields)].sort()).toEqual(
      [
        "cancelCronJobRun",
        "cronJobRuns",
        "finishedAt",
        "id",
        "runCronJob",
        "startedAt",
        "status",
      ].sort(),
    );
    for (const forbidden of [
      "name",
      "schedule",
      "command",
      "shell",
      "mechanism",
      "updateCronJob",
    ]) {
      expect(source.includes(forbidden)).toBe(false);
    }
  });

  it("keeps the history cursor opaque and exposes only run/cancel writes", () => {
    const operations = document.definitions.flatMap((definition) =>
      definition.kind === Kind.OPERATION_DEFINITION
        ? [
            {
              name: definition.name?.value,
              kind: definition.operation,
              variables: definition.variableDefinitions?.map(
                (variable) => variable.variable.name.value,
              ),
            },
          ]
        : [],
    );
    expect(operations).toEqual([
      {
        name: "MobileCronRuns",
        kind: "query",
        variables: ["serviceId", "cursor", "limit"],
      },
      {
        name: "MobileRunCronJob",
        kind: "mutation",
        variables: ["id"],
      },
      {
        name: "MobileCancelCronRun",
        kind: "mutation",
        variables: ["serviceId", "runId"],
      },
    ]);
  });
});
