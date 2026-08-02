/**
 * Pure current-user shaping for the drawer's personal footer. Validation and
 * the honest name/email fallbacks live here (no React, no fetch) so they can be
 * unit-tested and so no raw OAuth subject is ever surfaced as a friendly
 * identity.
 */

/** The validated subset of Render's `GET /v1/users` response we consume. */
export type CurrentUser = {
  /** Trimmed display name, or empty when the server returned none. */
  name: string;
  /** Trimmed email, or empty when the server returned none. */
  email: string;
};

function readString(source: Record<string, unknown>, key: string): string {
  const value = source[key];
  return typeof value === "string" ? value.trim() : "";
}

/**
 * Validate and normalize a `GET /v1/users` body into `{name,email}`. Render's
 * schema guarantees both fields are strings, so a non-object body is a real
 * protocol error; missing/blank members normalize to empty (an honest partial
 * identity), never to the id or a guess.
 */
export function parseCurrentUser(body: unknown): CurrentUser {
  if (typeof body !== "object" || body === null || Array.isArray(body)) {
    throw new Error("current-user response is not an object");
  }
  const source = body as Record<string, unknown>;
  return {
    name: readString(source, "name"),
    email: readString(source, "email"),
  };
}

/** How the footer should render an identity, after fallback resolution. */
export type IdentityDisplay = {
  /** Primary line: the name, else the email, else a translated status. */
  primary: string;
  /** Secondary line: the email when a name is shown; otherwise absent. */
  secondary: string | null;
  /** Up to two initials for the avatar, or an empty string for the glyph. */
  initials: string;
  /** Whether a real human name/email was resolved (vs. the status fallback). */
  identified: boolean;
};

/**
 * Derive avatar initials from a name (first letters of the first two words),
 * falling back to the email's first alphanumeric character. Returns "" when
 * neither yields a letter — the caller draws a neutral glyph rather than a
 * fragment of an opaque id.
 */
export function initialsFor(name: string, email: string): string {
  const words = name
    .split(/\s+/)
    .map((word) => word.trim())
    .filter(Boolean);
  const letters = words
    .slice(0, 2)
    .map((word) => word[0])
    .join("");
  if (letters) return letters.toUpperCase();
  const emailChar = email.replace(/[^a-z0-9]/gi, "")[0];
  return emailChar ? emailChar.toUpperCase() : "";
}

/**
 * Resolve the footer's display from a validated user and a translated
 * "Signed in" fallback. A name takes the primary line with email beneath; an
 * email-only response promotes the email; an empty response shows only the
 * honest signed-in status.
 */
export function identityDisplay(
  user: CurrentUser | null,
  signedInLabel: string,
): IdentityDisplay {
  const name = user?.name ?? "";
  const email = user?.email ?? "";
  if (name) {
    return {
      primary: name,
      secondary: email || null,
      initials: initialsFor(name, email),
      identified: true,
    };
  }
  if (email) {
    return {
      primary: email,
      secondary: null,
      initials: initialsFor("", email),
      identified: true,
    };
  }
  return {
    primary: signedInLabel,
    secondary: null,
    initials: "",
    identified: false,
  };
}
