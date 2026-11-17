import { afterEach, describe, expect, it, vi } from "vitest";
import {
  AuditExportTooLargeError,
  auditLogsToCSV,
  exportRangeDays,
  exportRangeTimes,
  fetchAuditLogRange,
  ownerAuditLogsPath,
  type RenderAuditLog,
} from "@/features/audit/lib/audit-csv";
import { config } from "@/config/config";

const log = (overrides: Partial<RenderAuditLog> = {}): RenderAuditLog => ({
  timestamp: "2026-11-10T09:00:00Z",
  event: "SuspendServiceEvent",
  status: "success",
  actor: { type: "user", id: "usr-1", email: "admin@example.com" },
  metadata: { service: "srv-1", targetName: "web" },
  ...overrides,
});

// w3/m305: Render's audit export — timestamp, actor, event, status, metadata —
// written from the REST list's own rows, so the file equals what
// GET /v1/owners/{ownerId}/audit-logs answers for the range.
describe("auditLogsToCSV", () => {
  it("writes Render's five columns, with REST's metadata map as JSON", () => {
    const csv = auditLogsToCSV([log()]);
    const [header, row] = csv.trimEnd().split("\r\n");
    expect(header).toBe("timestamp,actor,event,status,metadata");
    expect(row).toBe(
      '2026-11-10T09:00:00Z,admin@example.com,SuspendServiceEvent,success,"{""service"":""srv-1"",""targetName"":""web""}"',
    );
  });

  // The live check on production (2026-11-17) found the GraphQL-built file
  // writing `denied` where REST and Render's enum say `error`.
  it("keeps a denial as Render's error, with the outcome in the metadata", () => {
    const csv = auditLogsToCSV([
      log({
        event: "apps.Get",
        status: "error",
        metadata: { outcome: "denied", relation: "can_view" },
      }),
    ]);
    expect(csv.trimEnd().split("\r\n")[1]).toBe(
      '2026-11-10T09:00:00Z,admin@example.com,apps.Get,error,"{""outcome"":""denied"",""relation"":""can_view""}"',
    );
  });

  // w9/m450 + w9/099: Render's actor column is the person's email, or
  // `Deleted User ID#<id>` for a deleted user; a person whose email could not
  // be read keeps the bare id; a system row names nobody.
  it("writes the actor's email, a deleted user's marker, else the id, else nothing", () => {
    const csv = auditLogsToCSV([
      log(),
      log({
        actor: { type: "user", id: "usr-00000000000000000009" },
        actorDeleted: true,
      }),
      log({ actor: { type: "user", id: "usr-00000000000000000008" } }),
      log({ actor: { type: "system" } }),
    ]);
    const [, person, deleted, unread, system] = csv.trimEnd().split("\r\n");
    expect(person).toMatch(/^2026-11-10T09:00:00Z,admin@example\.com,/);
    expect(deleted).toMatch(
      /^2026-11-10T09:00:00Z,Deleted User ID#usr-00000000000000000009,/,
    );
    expect(unread).toMatch(/^2026-11-10T09:00:00Z,usr-00000000000000000008,/);
    expect(system).toMatch(/^2026-11-10T09:00:00Z,,SuspendServiceEvent,/);
  });

  it("quotes commas, quotes and newlines", () => {
    const csv = auditLogsToCSV([
      log({ actor: { type: "user", email: 'a,"b"\nc' } }),
    ]);
    expect(csv).toContain('"a,""b""\nc"');
  });

  it("keeps a tenant-named cell that a spreadsheet would run as a formula as text", () => {
    const csv = auditLogsToCSV([
      log({ actor: { type: "user", email: "=HYPERLINK(1)" }, event: "+1" }),
    ]);
    const row = csv.trimEnd().split("\r\n")[1];
    expect(
      row.startsWith("2026-11-10T09:00:00Z,'=HYPERLINK(1),'+1,success,"),
    ).toBe(true);
  });
});

describe("export range", () => {
  it("counts both ends of the range", () => {
    expect(exportRangeDays("2026-11-01", "2026-11-01")).toBe(1);
    expect(exportRangeDays("2026-08-13", "2026-11-10")).toBe(90);
    expect(exportRangeDays("2026-11-10", "2026-11-01")).toBeLessThan(1);
  });

  it("covers whole UTC days", () => {
    expect(exportRangeTimes("2026-11-01", "2026-11-10")).toEqual({
      startTime: "2026-11-01T00:00:00Z",
      endTime: "2026-11-10T23:59:59.999999Z",
    });
  });
});

const page = (from: number, size: number) =>
  Array.from({ length: size }, (_, i) => ({
    cursor: `aud-${from + i}`,
    auditLog: log({
      timestamp: `2026-11-10T09:00:${String(i % 60).padStart(2, "0")}Z`,
    }),
  }));

const respond = (body: unknown, status = 200) =>
  Promise.resolve(
    new Response(JSON.stringify(body), {
      status,
      headers: { "Content-Type": "application/json" },
    }),
  );

afterEach(() => {
  vi.unstubAllGlobals();
});

describe("fetchAuditLogRange", () => {
  it("pages REST's list by its cursor until a short page, with the range on every call", async () => {
    const fetch = vi
      .fn()
      .mockReturnValueOnce(respond(page(0, 1000)))
      .mockReturnValueOnce(respond(page(1000, 3)));
    vi.stubGlobal("fetch", fetch);
    const logs = await fetchAuditLogRange(
      ownerAuditLogsPath("tea-1"),
      "2026-11-01",
      "2026-11-10",
    );

    expect(logs).toHaveLength(1003);
    expect(logs[0]).toEqual(page(0, 1)[0].auditLog);
    expect(fetch).toHaveBeenCalledTimes(2);
    // The API base is empty when VITE_API_URL is unset (CI), so read the URL
    // against a placeholder origin and check its prefix separately.
    const url = (call: number) => fetch.mock.calls[call][0] as string;
    expect(
      url(0).startsWith(`${config.apiBaseUrl}/v1/owners/tea-1/audit-logs?`),
    ).toBe(true);
    const first = new URL(url(0), "http://api.invalid");
    expect(Object.fromEntries(first.searchParams)).toEqual({
      startTime: "2026-11-01T00:00:00Z",
      endTime: "2026-11-10T23:59:59.999999Z",
      limit: "1000",
    });
    expect(fetch.mock.calls[0][1]).toMatchObject({
      credentials: "include",
      cache: "no-store",
    });
    const second = new URL(url(1), "http://api.invalid");
    expect(second.searchParams.get("cursor")).toBe("aud-999");
  });

  it("fails on a refused read instead of writing a partial file", async () => {
    vi.stubGlobal(
      "fetch",
      vi.fn().mockReturnValue(respond({ message: "forbidden" }, 403)),
    );
    await expect(
      fetchAuditLogRange(
        ownerAuditLogsPath("tea-1"),
        "2026-11-01",
        "2026-11-10",
      ),
    ).rejects.toThrow("403");
  });

  it("stops reading once aborted", async () => {
    const controller = new AbortController();
    const fetch = vi.fn().mockImplementation(() => {
      controller.abort();
      return respond(page(0, 1000));
    });
    vi.stubGlobal("fetch", fetch);
    await expect(
      fetchAuditLogRange(
        ownerAuditLogsPath("tea-1"),
        "2026-11-01",
        "2026-11-10",
        controller.signal,
      ),
    ).rejects.toThrow();
    expect(fetch).toHaveBeenCalledTimes(1);
  });

  it("gives up with a named error past its row budget", async () => {
    const fetch = vi.fn().mockImplementation(() => respond(page(0, 1000)));
    vi.stubGlobal("fetch", fetch);
    await expect(
      fetchAuditLogRange(
        ownerAuditLogsPath("tea-1"),
        "2026-11-01",
        "2026-11-10",
      ),
    ).rejects.toBeInstanceOf(AuditExportTooLargeError);
    expect(fetch).toHaveBeenCalledTimes(20);
  });
});
