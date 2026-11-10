import { describe, expect, it, vi } from "vitest";
import type { AuditEvent } from "@/features/audit/types";
import {
  AuditExportTooLargeError,
  auditEventsToCSV,
  exportRangeDays,
  exportRangeTimes,
  fetchAuditRange,
} from "@/features/audit/lib/audit-csv";

const event = (overrides: Partial<AuditEvent> = {}): AuditEvent => ({
  id: "aud-1",
  timestamp: "2026-11-10T09:00:00Z",
  actor: "admin@example.com",
  actorMethod: "session",
  action: "apps.Suspend",
  status: "success",
  resource: "srv-1",
  targetName: "web",
  relation: "",
  oauthClientId: "",
  oauthAudience: "",
  oauthScopes: [],
  ...overrides,
});

// w3/m305: Render's audit export — timestamp, actor, event, status, metadata.
describe("auditEventsToCSV", () => {
  it("writes Render's five columns with the rest of the event as JSON metadata", () => {
    const csv = auditEventsToCSV([event()]);
    const [header, row] = csv.trimEnd().split("\r\n");
    expect(header).toBe("timestamp,actor,event,status,metadata");
    expect(row).toBe(
      '2026-11-10T09:00:00Z,admin@example.com,apps.Suspend,success,"{""resource"":""srv-1"",""targetName"":""web"",""actorMethod"":""session""}"',
    );
  });

  it("quotes commas, quotes and newlines", () => {
    const csv = auditEventsToCSV([event({ actor: 'a,"b"\nc' })]);
    expect(csv).toContain('"a,""b""\nc"');
  });

  it("keeps a tenant-named cell that a spreadsheet would run as a formula as text", () => {
    const csv = auditEventsToCSV([
      event({ actor: "=HYPERLINK(1)", action: "+1", status: "denied" }),
    ]);
    const row = csv.trimEnd().split("\r\n")[1];
    expect(
      row.startsWith("2026-11-10T09:00:00Z,'=HYPERLINK(1),'+1,denied,"),
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

const page = (from: number, size: number) => ({
  data: {
    auditLogs: Array.from({ length: size }, (_, i) => ({
      __typename: "AuditEvent",
      ...event({ id: `aud-${from + i}` }),
    })),
  },
});

describe("fetchAuditRange", () => {
  it("pages past the last id until a short page, with the range on every call", async () => {
    const query = vi
      .fn()
      .mockResolvedValueOnce(page(0, 1000))
      .mockResolvedValueOnce(page(1000, 3));
    const events = await fetchAuditRange(
      { query } as never,
      "tea-1",
      "2026-11-01",
      "2026-11-10",
    );

    expect(events).toHaveLength(1003);
    expect(query).toHaveBeenCalledTimes(2);
    expect(query.mock.calls[0][0].variables).toEqual({
      ownerId: "tea-1",
      startTime: "2026-11-01T00:00:00Z",
      endTime: "2026-11-10T23:59:59.999999Z",
      cursor: undefined,
      limit: 1000,
    });
    expect(query.mock.calls[1][0].variables.cursor).toBe("aud-999");
  });

  it("reads every page with no-cache, so an export leaves nothing in the cache", async () => {
    const query = vi.fn().mockResolvedValueOnce(page(0, 1));
    await fetchAuditRange({ query } as never, "tea-1", "2026-11-01", "2026-11-10");
    expect(query.mock.calls[0][0].fetchPolicy).toBe("no-cache");
  });

  it("stops reading once aborted", async () => {
    const controller = new AbortController();
    const query = vi.fn().mockImplementation(() => {
      controller.abort();
      return Promise.resolve(page(0, 1000));
    });
    await expect(
      fetchAuditRange({ query } as never, "tea-1", "2026-11-01", "2026-11-10", controller.signal),
    ).rejects.toThrow();
    expect(query).toHaveBeenCalledTimes(1);
  });

  it("gives up with a named error past its row budget", async () => {
    const query = vi
      .fn()
      .mockImplementation(() => Promise.resolve(page(0, 1000)));
    await expect(
      fetchAuditRange({ query } as never, "tea-1", "2026-11-01", "2026-11-10"),
    ).rejects.toBeInstanceOf(AuditExportTooLargeError);
    expect(query).toHaveBeenCalledTimes(20);
  });
});
