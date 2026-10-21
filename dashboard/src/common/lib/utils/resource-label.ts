// The free-form label rule bex-api applies to a service's name on create and
// to a Postgres or Key Value name on create and rename (w5/m153), as Render
// does: core.CheckLabel plus id.LooksLikeResourceID
// (lego/backend/internal/core/errors.go, lego/backend/internal/id/id.go).

/** The most Unicode code points a label may hold after trimming. */
export const MAX_LABEL_LENGTH = 100;

// The registered id prefixes (lego/backend/internal/id/id.go `kinds`), plus
// `env`, which LooksLikeResourceID also reserves. resource-label.test.ts reads
// id.go so this list cannot drift from it.
export const RESOURCE_ID_PREFIXES: ReadonlySet<string> = new Set([
  "tea",
  "srv",
  "dpg",
  "red",
  "cdm",
  "evg",
  "dep",
  "inv",
  "exp",
  "aud",
  "own",
  "evt",
  "crr",
  "ntf",
  "prj",
  "rgc",
  "blp",
  "evm",
  "whk",
  "whd",
  "wrl",
  "job",
  "ssk",
  "ssn",
  "bsr",
  "asi",
  "ags",
  "dsk",
  "wca",
  "cte",
  "exe",
  "gcs",
  "sbx",
  "rte",
  "hdr",
  "env",
]);

const ID_SUFFIX_RE = /^[0-9a-z]{20}$/;

/** Whether `value` is shaped like a resource id (`srv-` + 20 chars), which a
 *  selector could read as that id (w8/049). */
export function looksLikeResourceId(value: string): boolean {
  const dash = value.indexOf("-");
  if (dash < 0) return false;
  return (
    RESOURCE_ID_PREFIXES.has(value.slice(0, dash)) &&
    ID_SUFFIX_RE.test(value.slice(dash + 1))
  );
}

// Go's unicode.IsControl (C0, DEL, C1) plus the two line separators.
const CONTROL_RE = /[\p{Cc}\u2028\u2029]/u;

/**
 * Whether `value` is a label bex-api accepts once trimmed: non-empty, no
 * control characters or line separators, at most {@link MAX_LABEL_LENGTH}
 * code points, and not shaped like a resource id. Callers send
 * `value.trim()`.
 */
export function isValidLabel(value: string): boolean {
  if (CONTROL_RE.test(value)) return false;
  const label = value.trim();
  return (
    label !== "" &&
    [...label].length <= MAX_LABEL_LENGTH &&
    !looksLikeResourceId(label)
  );
}
