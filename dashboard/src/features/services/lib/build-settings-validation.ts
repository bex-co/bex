import type { en } from "@/i18n";

/**
 * The rules bex-api enforces on the build settings a service's Settings page
 * edits (lego/backend/internal/store/api.go ValidRootDir, NormalizeRootDir and
 * ValidGlob; apps/settings_check.go normalizeHealthCheckPath), checked in the
 * row so an invalid value is marked where it is, before Save and before the
 * "the service rebuilds and redeploys" confirmation, instead of a toast naming
 * the wire field (w21/045). The server stays authoritative; the vectors in
 * `__tests__/build-settings-validation.test.ts` are the same ones its Go test
 * pins, so the two cannot drift apart.
 */
type Reason = keyof typeof en;

const utf8 = new TextEncoder();
const MAX_PATH_BYTES = 512;

// Control characters the server refuses in a directory or a glob.
const CONTROL = /[\0\r\n\t]/;

/**
 * A relative path inside the repository, exactly as ValidRootDir judges it:
 * empty is fine, and `./x` is (only `..` and an empty segment are not), so
 * `a//b` and a trailing `/` are refused here. The Dockerfile Path is judged
 * as typed (after a trim): it names a file, so a trailing `/` is an error.
 */
export function relativePathError(raw: string): Reason | null {
  const value = raw.trim();
  if (value === "") return null;
  if (utf8.encode(value).length > MAX_PATH_BYTES) {
    return "services.buildPathTooLong";
  }
  if (CONTROL.test(value)) return "services.buildPathControlChar";
  if (value.includes("\\")) return "services.buildPathBackslash";
  if (value.startsWith("/")) return "services.buildPathAbsolute";
  for (const part of value.split("/")) {
    if (part === "..") return "services.buildPathDotDot";
    if (part === "") return "services.buildPathEmptySegment";
  }
  return null;
}

/**
 * NormalizeRootDir: the spellings of one directory the server maps to a single
 * form (`./`, `server/`, `./server`, `a//b`). A value it would refuse is
 * returned unchanged for the validity check below to name.
 */
function normalizeRootDir(value: string): string {
  if (value.startsWith("/") || /[\0\r\n\t\\]/.test(value)) return value;
  const kept: string[] = [];
  for (const part of value.split("/")) {
    if (part === "..") return value;
    if (part !== "" && part !== ".") kept.push(part);
  }
  return kept.join("/");
}

/**
 * The Root Directory and the Docker Build Context: judged after the server's
 * normalization, so `./` and `backend/` name directories (w8/292).
 */
export function rootDirError(raw: string): Reason | null {
  return relativePathError(normalizeRootDir(raw.trim()));
}

/**
 * A Build Filter pattern (ValidGlob): repository-relative, no `..` segment, no
 * backslash or control character, at most 512 bytes. An empty row is dropped on
 * save, so it is not an error. Whether doublestar can compile the pattern stays
 * server-side except for the certain case, a `[` that no `]` ever closes.
 */
export function buildFilterPatternError(raw: string): Reason | null {
  const value = raw.trim();
  if (value === "") return null;
  if (utf8.encode(value).length > MAX_PATH_BYTES) {
    return "services.buildPathTooLong";
  }
  if (CONTROL.test(value)) return "services.buildPathControlChar";
  if (value.includes("\\")) return "services.buildPathBackslash";
  if (value.startsWith("/")) return "services.buildPathAbsolute";
  if (value.split("/").includes("..")) return "services.buildPathDotDot";
  const open = value.lastIndexOf("[");
  if (open >= 0 && value.indexOf("]", open + 1) < 0) {
    return "services.buildFilterUnclosedClass";
  }
  return null;
}

/** The Health Check Path: empty (the TCP check) or rooted at `/`, after a trim. */
export function healthCheckPathError(raw: string): Reason | null {
  const value = raw.trim();
  return value !== "" && !value.startsWith("/")
    ? "services.healthCheckPathSlash"
    : null;
}
