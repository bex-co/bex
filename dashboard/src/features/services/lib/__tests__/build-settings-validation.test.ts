import { describe, expect, it } from "vitest";
import {
  buildFilterPatternError,
  healthCheckPathError,
  relativePathError,
  rootDirError,
} from "../build-settings-validation";

// w21/045: the vectors below are the server's. lego/backend/internal/store's
// TestRootDirProblemVectors pins the same table against ValidRootDir and
// NormalizeRootDir; change one and change the other.
const TOO_LONG = "services.buildPathTooLong";
const CONTROL = "services.buildPathControlChar";
const BACKSLASH = "services.buildPathBackslash";
const ABSOLUTE = "services.buildPathAbsolute";
const DOTDOT = "services.buildPathDotDot";
const EMPTY = "services.buildPathEmptySegment";

describe("Root Directory (judged after the server's normalization)", () => {
  it.each(["", "./", "./server", "server/", "a//b", "trail/", "a b", "a/./b"])(
    "accepts %j",
    (value) => {
      expect(rootDirError(value)).toBeNull();
    },
  );

  it.each([
    ["/abs", ABSOLUTE],
    ["../x", DOTDOT],
    ["a/../../b", DOTDOT],
    ["a/../b", DOTDOT],
    ["src\\app", BACKSLASH],
    ["a\tb", CONTROL],
    ["a\nb", CONTROL],
    ["a".repeat(513), TOO_LONG],
    // 171 Chinese characters are 513 bytes.
    ["中".repeat(171), TOO_LONG],
  ])("refuses %j", (value, reason) => {
    expect(rootDirError(value)).toBe(reason);
  });

  it("accepts a value at the byte limit", () => {
    expect(rootDirError("a".repeat(512))).toBeNull();
    expect(rootDirError("中".repeat(170))).toBeNull();
  });

  it("judges the trimmed value, as the dashboard sends it", () => {
    expect(rootDirError("  server  ")).toBeNull();
    expect(rootDirError("  ../x ")).toBe(DOTDOT);
  });
});

describe("Dockerfile Path (judged as typed)", () => {
  it.each(["", ".", "./Dockerfile", "docker/Dockerfile", "a b", "ctx"])(
    "accepts %j",
    (value) => {
      expect(relativePathError(value)).toBeNull();
    },
  );

  it.each([
    ["../Dockerfile", DOTDOT],
    ["../ctx", DOTDOT],
    ["/Dockerfile", ABSOLUTE],
    ["docker\\Dockerfile", BACKSLASH],
    // No normalization here: an empty segment is refused, where the Root
    // Directory would drop it.
    ["a//b", EMPTY],
    ["docker/", EMPTY],
    ["./", EMPTY],
    ["a".repeat(513), TOO_LONG],
    ["a\u0000b", CONTROL],
  ])("refuses %j", (value, reason) => {
    expect(relativePathError(value)).toBe(reason);
  });
});

describe("Build Filter patterns", () => {
  it.each([
    "",
    "   ",
    "src/**",
    " src/** ",
    "src/**/*.{js,ts}",
    "docs/[a-c]*.md",
    "**/*.go",
    // The doublestar compile stays server-side: only the certain case is marked.
    "a{b",
  ])("accepts %j", (value) => {
    expect(buildFilterPatternError(value)).toBeNull();
  });

  it.each([
    ["/abs/**", ABSOLUTE],
    ["../x", DOTDOT],
    ["src/../x", DOTDOT],
    ["src\\app", BACKSLASH],
    ["a".repeat(513), TOO_LONG],
    ["a\tb", CONTROL],
    ["[", "services.buildFilterUnclosedClass"],
    ["src/[abc", "services.buildFilterUnclosedClass"],
  ])("refuses %j", (value, reason) => {
    expect(buildFilterPatternError(value)).toBe(reason);
  });
});

describe("Health Check Path", () => {
  it.each(["", "   ", "/", "/healthz", "/h?x=1", "/with space", "//host/x"])(
    "accepts %j",
    (value) => {
      expect(healthCheckPathError(value)).toBeNull();
    },
  );

  it.each(["healthz", "  healthz", "h/z"])("refuses %j", (value) => {
    expect(healthCheckPathError(value)).toBe("services.healthCheckPathSlash");
  });
});
