import { describe, it, expect } from "vitest";
import {
  formatDateISO,
  formatDateTime,
  formatDateLong,
  formatNumber,
} from "../";

describe("Format Utility", () => {
  describe("formatDateISO", () => {
    it("should format date as YYYY-MM-DD", () => {
      expect(formatDateISO("2024-01-15")).toBe("2024-01-15");
      expect(formatDateISO("2024-12-31")).toBe("2024-12-31");
    });

    it("should zero-pad single digit months and days", () => {
      expect(formatDateISO("2024-03-05")).toBe("2024-03-05");
      expect(formatDateISO("2024-09-08")).toBe("2024-09-08");
    });

    it("should handle ISO 8601 format with timezone", () => {
      expect(formatDateISO("2024-06-15T00:00:00Z")).toBe("2024-06-15");
      expect(formatDateISO("2024-06-15T12:30:45Z")).toBe("2024-06-15");
    });

    it("should handle leap year dates", () => {
      expect(formatDateISO("2024-02-29")).toBe("2024-02-29");
    });

    it("should handle different years", () => {
      expect(formatDateISO("2020-05-15")).toBe("2020-05-15");
      expect(formatDateISO("2021-05-15")).toBe("2021-05-15");
      expect(formatDateISO("2024-05-15")).toBe("2024-05-15");
    });

    it("should return null for null input", () => {
      expect(formatDateISO(null)).toBeNull();
    });

    it("should return null for undefined input", () => {
      expect(formatDateISO(undefined)).toBeNull();
    });

    it("should return null for empty string", () => {
      expect(formatDateISO("")).toBeNull();
    });

    it("should return null for invalid date string", () => {
      expect(formatDateISO("invalid-date")).toBeNull();
    });

    it("should handle edge of year dates", () => {
      expect(formatDateISO("2024-01-01")).toBe("2024-01-01");
      expect(formatDateISO("2024-12-31")).toBe("2024-12-31");
    });

    it("should produce consistent output for the same date", () => {
      const result1 = formatDateISO("2024-06-15");
      const result2 = formatDateISO("2024-06-15");
      expect(result1).toBe(result2);
    });
  });

  // Zone-less inputs parse as local time, so the expected strings hold in any
  // test-runner timezone.
  describe("formatDateTime", () => {
    it("formats the dashboard's standard date + time", () => {
      expect(formatDateTime("2026-07-16T00:57:00")).toBe(
        "July 16, 2026 at 12:57 AM",
      );
      expect(formatDateTime("2026-07-16T15:05:00")).toBe(
        "July 16, 2026 at 3:05 PM",
      );
      expect(formatDateTime("2026-12-01T12:00:00")).toBe(
        "December 1, 2026 at 12:00 PM",
      );
    });

    it("returns null for missing or invalid input", () => {
      expect(formatDateTime(null)).toBeNull();
      expect(formatDateTime(undefined)).toBeNull();
      expect(formatDateTime("")).toBeNull();
      expect(formatDateTime("invalid-date")).toBeNull();
    });
  });

  describe("formatDateLong", () => {
    it("formats the date-only counterpart", () => {
      expect(formatDateLong("2026-07-16T00:57:00")).toBe("July 16, 2026");
      expect(formatDateLong("2026-02-03")).toBe("February 3, 2026");
    });

    it("returns null for missing or invalid input", () => {
      expect(formatDateLong(null)).toBeNull();
      expect(formatDateLong(undefined)).toBeNull();
      expect(formatDateLong("")).toBeNull();
      expect(formatDateLong("not-a-date")).toBeNull();
    });
  });

  describe("formatNumber", () => {
    it("adds thousands separators when renderCommas is true", () => {
      expect(formatNumber(1234567.89, true)).toBe("1,234,567.89");
    });

    it("omits thousands separators when renderCommas is false", () => {
      expect(formatNumber(1234567.89, false)).toBe("1234567.89");
    });

    it("keeps integers as-is without renderCommas", () => {
      expect(formatNumber(1234, false)).toBe("1234");
    });

    it("strips trailing zeros without renderCommas", () => {
      expect(formatNumber(1234.5, false)).toBe("1234.5");
    });
  });
});
