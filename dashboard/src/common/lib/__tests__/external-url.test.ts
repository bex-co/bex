import { describe, it, expect } from "vitest";
import { classifyHref, safeHttpHref } from "../external-url";

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
    for (const href of [
      "javascript:alert(1)",
      "data:text/html,x",
      "vbscript:x",
    ]) {
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

describe("safeHttpHref", () => {
  it("passes through absolute http(s) URLs", () => {
    expect(safeHttpHref("https://web.onbex.co")).toBe("https://web.onbex.co");
    expect(safeHttpHref("http://example.com/path?q=1")).toBe(
      "http://example.com/path?q=1",
    );
  });

  it("refuses active/unsupported and authority-obfuscating schemes", () => {
    for (const url of [
      "javascript:alert(document.domain)",
      "data:text/html,<script>alert(1)</script>",
      "vbscript:x",
      "//attacker.example", // protocol-relative
      "/\\attacker.example", // backslash-obfuscated authority
      "mailto:a@b.com", // not a browsable service URL
      "/relative/path",
      "not a url",
    ]) {
      expect(safeHttpHref(url), url).toBeUndefined();
    }
  });

  it("returns undefined for empty/nullish input", () => {
    expect(safeHttpHref(undefined)).toBeUndefined();
    expect(safeHttpHref(null)).toBeUndefined();
    expect(safeHttpHref("")).toBeUndefined();
  });
});
