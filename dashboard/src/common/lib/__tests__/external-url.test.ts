import { describe, it, expect } from "vitest";
import { classifyHref } from "../external-url";

describe("classifyHref", () => {
  it("classifies protocol-relative and backslash-obfuscated authorities as external", () => {
    for (const href of [
      "//attacker.example/login",
      "/\\attacker.example/login",
      "\\/attacker.example/login",
      "\\\\attacker.example/login",
    ]) {
      expect(classifyHref(href).isExternal, href).toBe(true);
    }
  });

  it("classifies http(s) absolute URLs as external regardless of scheme case", () => {
    expect(classifyHref("https://example.com").isExternal).toBe(true);
    expect(classifyHref("HTTP://example.com").isExternal).toBe(true);
  });

  it("keeps relative and root-relative paths internal", () => {
    for (const href of [
      "/services/abc",
      "docs/readme",
      "./x",
      "#section",
      "?q=1",
    ]) {
      const { safeHref, isExternal } = classifyHref(href);
      expect(isExternal, href).toBe(false);
      expect(safeHref, href).toBe(href);
    }
  });

  it("refuses active/unsupported schemes by returning no href", () => {
    for (const href of ["javascript:alert(1)", "data:text/html,x", "vbscript:x"]) {
      expect(classifyHref(href).safeHref, href).toBeUndefined();
    }
  });

  it("keeps mailto/tel as in-place links", () => {
    expect(classifyHref("mailto:a@b.com")).toEqual({
      safeHref: "mailto:a@b.com",
      isExternal: false,
    });
    expect(classifyHref("tel:+15551234")).toEqual({
      safeHref: "tel:+15551234",
      isExternal: false,
    });
  });

  it("returns the input unchanged for an empty destination", () => {
    expect(classifyHref(undefined)).toEqual({
      safeHref: undefined,
      isExternal: false,
    });
    expect(classifyHref("")).toEqual({ safeHref: "", isExternal: false });
  });
});
