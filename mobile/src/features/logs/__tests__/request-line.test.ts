import { requestLead } from "../request-line";
import type { LogLine } from "../types";

const line = (labels: Record<string, string>): LogLine => ({
  id: "1",
  message: 'clientIP="198.51.100.3" requestID="8ebfa3c3-8929-4885"',
  timestamp: "2026-11-11T00:00:00.000Z",
  labels: Object.entries(labels).map(([name, value]) => ({ name, value })),
});

describe("requestLead", () => {
  it("names a request line's method, status, host and path", () => {
    expect(
      requestLead(
        line({
          type: "request",
          method: "GET",
          statusCode: "503",
          host: "example.com",
          path: "/api/orders",
        }),
      ),
    ).toBe("[GET] 503 example.com/api/orders ");
  });

  it("is empty for an app line", () => {
    expect(requestLead(line({ type: "app", instance: "web-1" }))).toBe("");
  });
});
