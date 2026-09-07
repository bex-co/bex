import fs from "fs";
import path from "path";
import { parse, visit } from "graphql";
import {
  compactPostgresTableInsights,
  mergePostgresInsightState,
  postgresInsightFailure,
  postgresInsightState,
} from "../insights";

describe("Postgres mobile insights", () => {
  it("distinguishes empty, stale, degraded, and failed observations", () => {
    expect(
      postgresInsightState({
        hasData: false,
        failure: null,
        observedAt: null,
      }),
    ).toBe("loading");
    expect(
      postgresInsightState({
        hasData: true,
        failure: null,
        observedAt: 1_000,
        now: 65_999,
        staleAfterMs: 65_000,
      }),
    ).toBe("current");
    expect(
      postgresInsightState({
        hasData: true,
        failure: null,
        observedAt: 1_000,
        now: 66_000,
        staleAfterMs: 65_000,
      }),
    ).toBe("stale");
    expect(
      postgresInsightState({
        hasData: true,
        failure: "transport-error",
        observedAt: 1_000,
      }),
    ).toBe("degraded");
    expect(
      postgresInsightState({
        hasData: false,
        failure: "transport-error",
        observedAt: null,
      }),
    ).toBe("transport-error");
    expect(
      postgresInsightState({
        hasData: false,
        failure: "source-unavailable",
        observedAt: null,
      }),
    ).toBe("source-unavailable");
    expect(
      postgresInsightState({
        hasData: true,
        failure: "source-unavailable",
        observedAt: 1_000,
      }),
    ).toBe("degraded");
    for (const observedAt of [Number.NaN, Number.POSITIVE_INFINITY]) {
      expect(
        postgresInsightState({
          hasData: true,
          failure: null,
          observedAt,
        }),
      ).toBe("loading");
    }
  });

  it("distinguishes the stable missing-source signal from transport errors", () => {
    expect(
      postgresInsightFailure(
        new Error("metrics source not configured: prometheus disabled"),
      ),
    ).toBe("source-unavailable");
    expect(
      postgresInsightFailure({
        graphQLErrors: [{ extensions: { code: "METRICS_UNAVAILABLE" } }],
      }),
    ).toBe("source-unavailable");
    expect(postgresInsightFailure(new Error("Network request failed"))).toBe(
      "transport-error",
    );
    expect(postgresInsightFailure(null)).toBe(null);
  });

  it("preserves composite insight-state precedence", () => {
    const cases = [
      ["source-unavailable", "loading", "source-unavailable"],
      ["source-unavailable", "current", "degraded"],
      ["source-unavailable", "transport-error", "transport-error"],
      ["empty", "current", "degraded"],
      ["empty", "stale", "empty"],
      ["stale", "current", "stale"],
    ] as const;
    for (const [left, right, expected] of cases) {
      expect(mergePostgresInsightState(left, right)).toBe(expected);
      expect(mergePostgresInsightState(right, left)).toBe(expected);
    }
  });

  it("joins compact table sizes and scans while keeping missing data honest", () => {
    expect(
      compactPostgresTableInsights(
        [
          { schema: "public", name: "orders", sizePretty: "12 MB" },
          { schema: "public", name: "users", sizePretty: "4 MB" },
        ],
        [
          {
            schema: "public",
            name: "users",
            seqScans: 20,
            indexScans: 3,
            deadRows: 5,
          },
          {
            schema: "public",
            name: "orders",
            seqScans: 1,
            indexScans: 30,
            deadRows: 0,
          },
        ],
      ),
    ).toEqual([
      {
        key: "public.users",
        label: "public.users",
        sizePretty: "4 MB",
        seqScans: 20,
        indexScans: 3,
        deadRows: 5,
      },
      {
        key: "public.orders",
        label: "public.orders",
        sizePretty: "12 MB",
        seqScans: 1,
        indexScans: 30,
        deadRows: 0,
      },
    ]);
    expect(
      compactPostgresTableInsights(
        [{ schema: "public", name: "size_only", sizePretty: "1 MB" }],
        [],
      )[0],
    ).toEqual({
      key: "public.size_only",
      label: "public.size_only",
      sizePretty: "1 MB",
      seqScans: null,
      indexScans: null,
      deadRows: null,
    });
    expect(
      compactPostgresTableInsights(
        [],
        [
          {
            schema: "public",
            name: "invalid_counters",
            seqScans: -1,
            indexScans: Number.NaN,
            deadRows: null,
          },
        ],
      )[0],
    ).toEqual({
      key: "public.invalid_counters",
      label: "public.invalid_counters",
      sizePretty: null,
      seqScans: null,
      indexScans: null,
      deadRows: null,
    });
  });

  it("requests only read-only, non-credential Postgres fields", () => {
    const document = fs.readFileSync(
      path.resolve(process.cwd(), "src/features/postgres/api/insights.graphql"),
      "utf8",
    );
    const fields: string[] = [];
    visit(parse(document), {
      Field: (node) => void fields.push(node.name.value),
    });
    expect(document).toContain('kind: "DATABASE"');
    expect(document).toContain('name: "DISK"');
    expect(document).toContain('name: "DISK_CAPACITY"');
    expect(document).toContain('name: "DB_CONNECTIONS"');
    for (const forbidden of [
      "query",
      "userName",
      "password",
      "databaseConnectionInfo",
      "internalConnectionString",
      "externalConnectionString",
      "psqlCommand",
      "databaseParameterOverrides",
      "databaseRecoveryInfo",
      "databaseIpAllowList",
      "plan",
    ]) {
      expect(fields).not.toContain(forbidden);
    }
    expect(document).not.toContain("REPLICATION_LAG");
    expect((document.match(/^mutation\s/gm) ?? []).length).toBe(0);
  });
});
