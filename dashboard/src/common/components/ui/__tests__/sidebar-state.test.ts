import { describe, it, expect } from "vitest";
import {
  clampSidebarWidth,
  decodeSidebarState,
  encodeSidebarState,
  SIDEBAR_DEFAULT_WIDTH_PX,
  SIDEBAR_MAX_WIDTH_PX,
  SIDEBAR_MIN_WIDTH_PX,
} from "../sidebar-state";

describe("sidebar-state codec (w2/m63)", () => {
  describe("clampSidebarWidth", () => {
    it("keeps in-band widths and rounds fractional px", () => {
      expect(clampSidebarWidth(256.4)).toBe(256);
      expect(clampSidebarWidth(256.6)).toBe(257);
    });
    it("clamps below min and above max", () => {
      expect(clampSidebarWidth(10)).toBe(SIDEBAR_MIN_WIDTH_PX);
      expect(clampSidebarWidth(9999)).toBe(SIDEBAR_MAX_WIDTH_PX);
    });
  });

  describe("encode → decode round-trips the signed width", () => {
    it("expanded encodes a positive width", () => {
      expect(encodeSidebarState({ open: true, width: 300 })).toBe("300");
      expect(decodeSidebarState("300")).toEqual({ open: true, width: 300 });
    });
    it("collapsed encodes the negated remembered width", () => {
      expect(encodeSidebarState({ open: false, width: 300 })).toBe("-300");
      expect(decodeSidebarState("-300")).toEqual({ open: false, width: 300 });
    });
    it("clamps out-of-band widths on the way in and out", () => {
      expect(encodeSidebarState({ open: true, width: 5000 })).toBe(
        String(SIDEBAR_MAX_WIDTH_PX),
      );
      expect(decodeSidebarState("5000")).toEqual({
        open: true,
        width: SIDEBAR_MAX_WIDTH_PX,
      });
    });
  });

  describe("decodeSidebarState edge cases", () => {
    it("returns null for no / empty / zero / non-numeric values", () => {
      expect(decodeSidebarState(null)).toBeNull();
      expect(decodeSidebarState(undefined)).toBeNull();
      expect(decodeSidebarState("")).toBeNull();
      expect(decodeSidebarState("  ")).toBeNull();
      expect(decodeSidebarState("0")).toBeNull();
      expect(decodeSidebarState("nonsense")).toBeNull();
    });
    it("tolerates the legacy boolean cookie without crashing", () => {
      expect(decodeSidebarState("true")).toEqual({
        open: true,
        width: SIDEBAR_DEFAULT_WIDTH_PX,
      });
      expect(decodeSidebarState("false")).toEqual({
        open: false,
        width: SIDEBAR_DEFAULT_WIDTH_PX,
      });
    });
  });
});
