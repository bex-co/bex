import { formatTimestamp, humanizeToken } from "../format-util";

describe("humanizeToken", () => {
  it("presents backend enum tokens as readable labels", () => {
    expect(humanizeToken("web_service")).toBe("Web service");
    expect(humanizeToken("update_in_progress")).toBe("Update in progress");
    expect(humanizeToken("available")).toBe("Available");
  });

  it("leaves already-presentable values alone", () => {
    expect(humanizeToken("Running")).toBe("Running");
    expect(humanizeToken("PostgreSQL 18")).toBe("PostgreSQL 18");
  });

  it("returns the input when there is nothing to present", () => {
    expect(humanizeToken("")).toBe("");
    expect(humanizeToken("___")).toBe("___");
  });
});

describe("formatTimestamp", () => {
  it("accepts ISO strings and epoch milliseconds alike", () => {
    const iso = "2026-08-02T12:34:00.000Z";
    expect(formatTimestamp(Date.parse(iso))).toBe(formatTimestamp(iso));
  });

  it("fails honestly for invalid values", () => {
    expect(formatTimestamp("not-a-date")).toBe("—");
  });
});
