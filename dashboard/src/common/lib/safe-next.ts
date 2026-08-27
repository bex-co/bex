/**
 * Same-origin normalizer for a login `?next=` (and any Ory `return_to`) target.
 *
 * The login route accepts an arbitrary `next` string. It is fed to Kratos as the
 * flow's `return_to` (Kratos 30x's a browser to it) and to `navigate({ to: "/",
 * href: next })` — and TanStack Router treats an external http(s) `href` as a
 * full-page navigation. So an unchecked `next` is an open redirect to any origin
 * right after a trusted auth endpoint. Every caller must route the raw value
 * through here first.
 *
 * The result is ALWAYS a same-origin, root-relative `pathname+search+hash`
 * (never an origin), or `/` when the input is anything else. Only a value that
 *
 *   - is a non-empty string,
 *   - begins with a single "/" that does not start a protocol-relative ("//")
 *     or backslash-obfuscated ("/\") authority, and
 *   - resolves to *this* origin,
 *
 * survives. That rejects absolute URLs (`https://evil.com`), protocol-relative
 * (`//evil.com`), percent-encoded (`%2f%2fevil.com`, `https:%2f%2f…`), backslash
 * (`/\evil.com`, `\/\/evil.com`), and mixed-case-scheme values — all collapse to
 * `/` — while preserving legitimate internal routes with their query and hash.
 *
 * The leading-slash whitelist runs before the URL parser on purpose: the WHATWG
 * parser normalizes backslashes to forward slashes for special schemes, so
 * `/\evil.com` parses with `evil.com` as its authority — the origin check below
 * would still catch it, but rejecting on the raw string is the clearer guard.
 */
export function safeNext(
  next: string | null | undefined,
  origin: string = defaultOrigin(),
): string {
  const FALLBACK = "/";
  if (typeof next !== "string" || next.length === 0) return FALLBACK;

  // Only a single leading slash that isn't the start of an authority.
  if (next[0] !== "/" || next[1] === "/" || next[1] === "\\") return FALLBACK;

  let base: URL;
  let resolved: URL;
  try {
    base = new URL(origin);
    resolved = new URL(next, base);
  } catch {
    return FALLBACK;
  }
  // Belt and suspenders: a leading-"/" value can still resolve cross-origin
  // under the backslash rule, so require the resolved origin to match.
  if (resolved.origin !== base.origin) return FALLBACK;

  return `${resolved.pathname}${resolved.search}${resolved.hash}`;
}

/**
 * The current location as a root-relative href (`pathname + search + hash`) —
 * the value handed to a login `?next=` so sign-in returns the user to exactly
 * where they were. Browser-only; empty string on the server (no `window`).
 */
export function currentHref(): string {
  if (typeof window === "undefined") return "";
  const { pathname, search, hash } = window.location;
  return pathname + search + hash;
}

/**
 * The origin to resolve against. In the browser this is the real app origin; on
 * the server (SSR render of the login page) `window` is absent, but a
 * placeholder is safe because only the relative `pathname+search+hash` is ever
 * returned — so the server and client agree, avoiding a hydration mismatch.
 */
function defaultOrigin(): string {
  if (typeof window !== "undefined" && window.location?.origin) {
    return window.location.origin;
  }
  return "http://localhost";
}
