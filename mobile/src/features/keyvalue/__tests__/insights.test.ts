import fs from "fs";
import path from "path";
import { Kind, parse } from "graphql";
import {
  buildKeyValueInsightSnapshot,
  keyValueConnectionHealth,
  keyValueMetricFailure,
} from "../insights";

describe("Key Value mobile read-only insights", () => {
  it("derives connection availability without treating suspension as healthy", () => {
    expect(
      keyValueConnectionHealth({
        status: "available",
        suspended: "not_suspended",
      }),
    ).toBe("available");
    expect(
      keyValueConnectionHealth({ status: "available", suspended: "suspended" }),
    ).toBe("suspended");
    expect(
      keyValueConnectionHealth({ status: "creating", suspended: false }),
    ).toBe("creating");
    expect(
      keyValueConnectionHealth({ status: "unavailable", suspended: false }),
    ).toBe("unavailable");
    expect(
      keyValueConnectionHealth({ status: "ready", suspended: false }),
    ).toBe("unknown");
  });

  it("keeps real latest samples and never invents capacity or freshness", () => {
    const snapshot = buildKeyValueInsightSnapshot({
      disk: [series("bytes", [["2026-08-02T10:00:00Z", 25]])],
      diskCapacity: [series("bytes", [["2026-08-02T10:00:01Z", 100]])],
      memory: [series("bytes", [["2026-08-02T10:00:02Z", 42]])],
      connections: [series("count", [["2026-08-02T10:00:03Z", 3]])],
    });
    expect(snapshot.diskUsedPercent).toBe(25);
    expect(snapshot.latestAt).toBe("2026-08-02T10:00:03.000Z");
    expect(snapshot.connections.current).toBe(3);

    const absent = buildKeyValueInsightSnapshot({
      disk: [series("bytes", [["2026-08-02T10:00:00Z", 25]])],
      diskCapacity: [],
    });
    expect(absent.diskUsedPercent).toBe(null);
    expect(buildKeyValueInsightSnapshot(undefined).latestAt).toBe(null);
  });

  it("classifies only the failed alias and preserves a partial payload", () => {
    const error = {
      errors: [
        {
          message: "metrics source not configured",
          path: ["memory"],
        },
      ],
    };
    expect(keyValueMetricFailure(error, "memory", false)).toBe("unavailable");
    expect(keyValueMetricFailure(error, "connections", false)).toBe(null);
    expect(keyValueMetricFailure(error, "memory", true)).toBe("unavailable");
    expect(
      keyValueMetricFailure(new Error("network failed"), "memory", true),
    ).toBe("error");
  });

  it("queries only the four safe read-only metric aliases", () => {
    const document = fs.readFileSync(
      path.resolve(process.cwd(), "src/features/keyvalue/api/insights.graphql"),
      "utf8",
    );
    const parsed = parse(document);
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
    expect(
      operation?.kind === Kind.OPERATION_DEFINITION
        ? operation.selectionSet.selections.flatMap((selection) =>
            selection.kind === Kind.FIELD ? [selection.alias?.value] : [],
          )
        : [],
    ).toEqual(["disk", "diskCapacity", "memory", "connections"]);
    expect(
      /connectionString|password|secret|ipAllowList|maxmemory|persistence|plan|mutation/i.test(
        document.replace(/#.*$/gm, ""),
      ),
    ).toBe(false);
  });
});

function series(
  unit: string,
  values: Array<[string, number]>,
): {
  unit: string;
  labels: null;
  values: Array<{ time: string; value: number }>;
} {
  return {
    unit,
    labels: null,
    values: values.map(([time, value]) => ({ time, value })),
  };
}
