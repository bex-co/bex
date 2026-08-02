import fs from "fs";
import path from "path";
import { parse, visit } from "graphql";
import {
  compactPostgresTableInsights,
  postgresInsightState,
  summarizePostgresProcesses,
} from "../insights";

describe("Postgres mobile insights", () => {
  it("distinguishes empty, stale, degraded, and unavailable observations", () => {
    expect(
      postgresInsightState({
        hasData: false,
        hasError: false,
        observedAt: null,
      }),
    ).toBe("loading");
    expect(
      postgresInsightState({
        hasData: true,
        hasError: false,
        observedAt: 1_000,
        now: 65_999,
        staleAfterMs: 65_000,
      }),
    ).toBe("current");
    expect(
      postgresInsightState({
        hasData: true,
        hasError: false,
        observedAt: 1_000,
        now: 66_000,
        staleAfterMs: 65_000,
      }),
    ).toBe("stale");
    expect(
      postgresInsightState({
        hasData: true,
        hasError: true,
        observedAt: 1_000,
      }),
    ).toBe("degraded");
    expect(
      postgresInsightState({
        hasData: false,
        hasError: true,
        observedAt: null,
      }),
    ).toBe("unavailable");
  });

  it("summarizes processes without retaining query text or database users", () => {
    expect(
      summarizePostgresProcesses([
        { state: "active", waitEventType: "Lock", durationSeconds: 12 },
        { state: "idle", waitEventType: "", durationSeconds: 2 },
        { state: "active", waitEventType: null, durationSeconds: null },
      ]),
    ).toEqual({ total: 3, active: 2, waiting: 1, longestSeconds: 12 });
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
