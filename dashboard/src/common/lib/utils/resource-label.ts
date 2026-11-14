// The free-form label rule bex-api applies to a service's name on create and
// to a Postgres or Key Value name on create and rename (w5/m153), as Render
// does: core.CheckLabel (lego/backend/internal/core/errors.go). A label shaped
// like a resource id is accepted, as on Render (w5/m118 reversed w8/049).

/** The most Unicode code points a label may hold after trimming. */
export const MAX_LABEL_LENGTH = 100;

// Go's unicode.IsControl (C0, DEL, C1) plus the two line separators.
const CONTROL_RE = /[\p{Cc}\u2028\u2029]/u;

/**
 * Whether `value` is a label bex-api accepts once trimmed: non-empty, no
 * control characters or line separators, and at most
 * {@link MAX_LABEL_LENGTH} code points. Callers send `value.trim()`, so the
 * rule is judged on that: a newline at an edge is only whitespace, while a NEL
 * (U+0085), which `trim` leaves, is the control character it is.
 */
export function isValidLabel(value: string): boolean {
  const label = value.trim();
  if (CONTROL_RE.test(label)) return false;
  return label !== "" && [...label].length <= MAX_LABEL_LENGTH;
}

/**
 * Whether a name field holds text the label rule refuses (control characters,
 * or more than 100 code points). Blank is not "invalid" here: each form already
 * treats an empty name as simply not submittable (w21/036).
 */
export function isInvalidLabel(value: string): boolean {
  return value.trim() !== "" && !isValidLabel(value);
}
